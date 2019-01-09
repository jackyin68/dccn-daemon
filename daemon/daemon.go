package daemon

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Ankr-network/dccn-daemon/task"
	pb "github.com/Ankr-network/dccn-rpc/protocol_new/k8s"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type taskCtx struct {
	*pb.Task
	stream pb.Dccnk8S_K8TaskClient
	ctx    context.Context
}

func ServeTask(cfgpath, namespace, ingressHost, hubServer, dcName,
	tendermintServer, tendermintWsEndpoint string) error {
	runner, err := task.NewRunner(cfgpath, namespace, ingressHost)
	if err != nil {
		return err
	}

	go taskMetering(runner, dcName, namespace, tendermintServer, tendermintWsEndpoint)

	var taskCh = make(chan *taskCtx) // block chan, serve single task one time
	go taskOperator(runner, dcName, taskCh)
	return taskReciver(runner, hubServer, dcName, taskCh)
}

func taskMetering(r *task.Runner, dcName, namespace, server, wsEndpoint string) {
	once := &sync.Once{}
	tick := time.Tick(30 * time.Second)
	for range tick {
		metering, err := r.Metering()
		if err != nil {
			glog.Errorln("client fail to get metering:", err)
			continue
		}

		if err := Broadcast(server, wsEndpoint, TendermintKey(dcName, namespace), metering); err != nil {
			if strings.Contains(err.Error(), "Tx already exists in cache") {
				glog.V(3).Infoln(err)
			} else {
				glog.Errorln("client fail to marshal metering:", err)
			}
			continue
		}

		once.Do(func() {
			glog.Infoln("Metering boradcast started.")
		})
	}
}

func taskReciver(r *task.Runner, hubServer, dcName string, taskCh chan<- *taskCtx) error {
	// try once to test connection, all tests should finish in 5s
	stream, closeStream, err := dialStream(5*time.Second, hubServer)
	if err != nil {
		return err
	}
	closeStream()
	glog.Infoln("Task reciver started.")

	redial := true
	for {
		if redial {
			stream, closeStream, err = dialStream(0, hubServer)
			if err != nil {
				glog.Errorln("client fail to receive task:", err)
				continue
			}

			// regist dc
			if err := heartBeat(r, dcName, stream); err != nil {
				closeStream()
			} else {
				redial = false
			}
		}

		if in, err := stream.Recv(); err == io.EOF {
			redial = true
			closeStream()
			continue

		} else if err != nil {
			redial = true
			closeStream()
			time.Sleep(5 * time.Second)
			glog.Errorln("Failed to receive task:", err)

		} else {
			glog.V(1).Infof("new task %s: %v", in.Type, in)
			taskCh <- &taskCtx{
				Task:   in,
				stream: stream,
			}
		}
	}
}

func taskOperator(r *task.Runner, dcName string, taskCh <-chan *taskCtx) {
	glog.Infoln("Task operator started.")
	for {
		task, ok := <-taskCh
		if !ok {
			return
		}

		var message = pb.K8SMessage{
			Datacenter: dcName,
			Taskname:   task.Name,
			Type:       task.Type,
		}

		switch task.Type {
		case HeartBeat.String():
			heartBeat(r, dcName, task.stream)

		case NewTask.String():
			images := strings.Split(task.Image, ",")
			if err := r.CreateTasks(task.Name, images...); err != nil {
				glog.V(1).Infoln(err)
				message.Status = StartFailure.String()
				message.Report = err.Error()
			} else {
				message.Status = StartSuccess.String()
			}

			send(task.stream, &message)

		case UpdateTask.String():
			// FIXME: hard code for no definition in protobuf
			if err := r.UpdateTask(task.Name, task.Image, 2, 80, 80); err != nil {
				glog.V(1).Infoln(err)
				message.Status = UpdateFailure.String()
				message.Report = err.Error()
			} else {
				message.Status = UpdateSuccess.String()
			}

			send(task.stream, &message)

		case CancelTask.String():
			if err := r.CancelTask(task.Name); err != nil {
				glog.V(1).Infoln(err)
				message.Status = CancelFailure.String()
				message.Report = err.Error()
			} else {
				message.Status = Cancelled.String()
			}

			send(task.stream, &message)
		}
	}
}

func heartBeat(r *task.Runner, dcName string, stream pb.Dccnk8S_K8TaskClient) error {
	var message = pb.K8SMessage{
		Datacenter: dcName,
		Taskname:   "",
		Type:       HeartBeat.String(),
	}

	tasks, err := r.ListTask()
	if err != nil {
		glog.V(1).Infoln(err)
		message.Report = err.Error()
	} else {
		message.Report = strings.Join(tasks, "\n")
	}

	return send(stream, &message)
}

func dialStream(timeout time.Duration, hubServer string) (pb.Dccnk8S_K8TaskClient, func(), error) {
	var cancel context.CancelFunc
	var ctx = context.Background()
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}

	conn, err := grpc.DialContext(ctx, hubServer, grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{ // TODO: dynamic config in config file
			Time:    20 * time.Second,
			Timeout: 5 * time.Second,
		}))
	if err != nil {
		cancel()
		return nil, nil, errors.Wrapf(err, "dail ankr hub %s", hubServer)
	}

	client := pb.NewDccnk8SClient(conn)
	stream, err := client.K8Task(ctx)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		conn.Close()
		return nil, nil, errors.Wrap(err, "listen k8s task")
	}

	return stream, func() {
		cancel()
		stream.CloseSend()
		conn.Close()
	}, nil
}

func send(stream pb.Dccnk8S_K8TaskClient, msg *pb.K8SMessage) error {
	if err := stream.Send(msg); err != nil {
		glog.V(2).Infof("send (%v) fail: %s", *msg, err)
		return err
	}

	glog.V(3).Infof("send %s success", msg.Type)
	return nil
}
