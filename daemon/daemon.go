package daemon

import (
	"context"
	"fmt"
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

func ServeTask(cfgpath, namespace, ingressHost, hubServer, dcName,
	tendermintServer, tendermintWsEndpoint string) error {
	runner, err := task.NewRunner(cfgpath, namespace, ingressHost)
	if err != nil {
		return err
	}

	go taskMetering(runner, dcName, namespace, tendermintServer, tendermintWsEndpoint)

	var taskCh = make(chan *pb.Task) // block chan, serve single task one time
	go taskHandler(runner, dcName, taskCh)
	return taskReciver(runner, hubServer, taskCh)
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

var stream pb.Dccnk8S_K8TaskClient // interface is refer type, CPU keep atomic action, lock free

func taskReciver(r *task.Runner, hubServer string, taskCh chan<- *pb.Task) error {
	dialStream := func(timeout time.Duration) (pb.Dccnk8S_K8TaskClient, func(), error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)

		conn, err := grpc.DialContext(context.Background(), hubServer, grpc.WithInsecure(),
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
			cancel()
			conn.Close()
			return nil, nil, errors.Wrap(err, "listen k8s task")
		}

		return stream, func() {
			cancel()
			stream.CloseSend()
			conn.Close()
		}, nil
	}

	// try once to test connection, all tests should finish in 5s
	_, closeStream, err := dialStream(5 * time.Second)
	if err != nil {
		return err
	}
	closeStream()
	glog.Infoln("Task reciver started.")

	redial := true
	for {
		if redial {
			stream, closeStream, err = dialStream(1000 * time.Second)
			if err != nil {
				glog.Errorln("client fail to receive task:", err)
				continue
			}

			redial = false
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
			taskCh <- in
		}
	}
}

func taskHandler(r *task.Runner, dcName string, taskCh <-chan *pb.Task) {
	//send heartBeat to register cluster
	tick := time.Tick(200 * time.Millisecond)
	for range tick {
		glog.V(3).Infof("%v", stream)
		if stream != nil {
			break
		}
	}
	heartBeat(r, dcName, stream)
	glog.Infoln("Task reciver started.")

	tick = time.Tick(30 * time.Second)
	for {
		select {
		case <-tick:
			go heartBeat(r, dcName, stream)

		case task, ok := <-taskCh:
			if !ok {
				return
			}

			var message = pb.K8SMessage{
				Datacenter: dcName,
				Taskname:   task.Name,
				Type:       task.Type,
			}
			taskName := fmt.Sprintf("%s_%d", task.Name, task.Taskid)

			switch task.Type {
			case HeartBeat.String():
				heartBeat(r, dcName, stream)

			case NewTask.String():
				images := strings.Split(task.Image, ",")
				if err := r.CreateTasks(taskName, images...); err != nil {
					glog.V(1).Infoln(err)
					message.Status = StartFailure.String()
					message.Report = err.Error()
				} else {
					message.Status = StartSuccess.String()
				}

				send(stream, &message)

			case UpdateTask.String():
				// FIXME: hard code for no definition in protobuf
				if err := r.UpdateTask(taskName, task.Image, 2, 80, 80); err != nil {
					glog.V(1).Infoln(err)
					message.Status = UpdateFailure.String()
					message.Report = err.Error()
				} else {
					message.Status = UpdateSuccess.String()
				}

				send(stream, &message)

			case CancelTask.String():
				if err := r.CancelTask(taskName); err != nil {
					glog.V(1).Infoln(err)
					message.Status = CancelFailure.String()
					message.Report = err.Error()
				} else {
					message.Status = Cancelled.String()
				}

				send(stream, &message)
			}
		}
	}
}

func heartBeat(r *task.Runner, dcName string, stream pb.Dccnk8S_K8TaskClient) {
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

	send(stream, &message)
}

func send(stream pb.Dccnk8S_K8TaskClient, msg *pb.K8SMessage) {
	if err := stream.Send(msg); err != nil {
		glog.V(2).Infof("send (%v) fail: %s", *msg, err)
		return
	}
	glog.V(3).Infof("send %s success", msg.Type)
}
