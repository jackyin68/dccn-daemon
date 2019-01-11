package daemon

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	common_proto "github.com/Ankr-network/dccn-common/protos/common"
	grpc_dcmgr "github.com/Ankr-network/dccn-common/protos/dcmgr/v1/grpc"
	"github.com/Ankr-network/dccn-daemon/task"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type taskCtx struct {
	*common_proto.Event
	stream grpc_dcmgr.DCStreamer_ServerStreamClient
	ctx    context.Context
}

// ServeTask will serve the task metering with blockchain logic
func ServeTask(cfgpath, namespace, ingressHost, hubServer, dcName,
	tendermintServer, tendermintWsEndpoint string) error {
	client, err := task.NewClient(cfgpath, namespace, ingressHost)
	if err != nil {
		return err
	}

	go taskMetering(client, dcName, namespace, tendermintServer, tendermintWsEndpoint)

	var taskCh = make(chan *taskCtx) // block chan, serve single task one time
	go taskOperator(client, dcName, taskCh)
	return taskReciver(client, hubServer, dcName, taskCh)
}

func taskMetering(c *task.Client, dcName, namespace, server, wsEndpoint string) {
	once := &sync.Once{}
	tick := time.Tick(30 * time.Second)
	for range tick {
		metering, err := c.Metering()
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

func taskReciver(c *task.Client, hubServer, dcName string, taskCh chan<- *taskCtx) error {
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
				time.Sleep(5 * time.Second)
				continue
			}

			// regist dc
			if err := heartBeat(c, dcName, stream); err != nil {
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
			glog.V(1).Infof("new task %s: %v", in.EventType, in)
			taskCh <- &taskCtx{
				Event:  in,
				stream: stream,
			}
		}
	}
}

func taskOperator(c *task.Client, dcName string, taskCh <-chan *taskCtx) {
	glog.Infoln("Task operator started.")
	for {
		chTask, ok := <-taskCh
		if !ok {
			return
		}

		task := chTask.GetTask()

		switch chTask.EventType {
		case common_proto.Operation_HEARTBEAT:
			heartBeat(c, dcName, chTask.stream)

		case common_proto.Operation_TASK_CREATE:
			images := strings.Split(task.Image, ",")
			task.Status = common_proto.TaskStatus_RUNNING
			if err := c.CreateTasks(task.Name, images...); err != nil {
				glog.V(1).Infoln(err)
				task.Status = common_proto.TaskStatus_START_FAILED
				chTask.Report = err.Error()
			}
			send(chTask.stream, &common_proto.Event{
				EventType: common_proto.Operation_TASK_CREATE,
				OpMessage: &common_proto.Event_TaskFeedback{
					TaskFeedback: &common_proto.TaskFeedback{TaskId: task.Id, Url: task.Url, DataCenter: task.DataCenterId, Report: chTask.Report}}})

		case common_proto.Operation_TASK_UPDATE:
			// FIXME: hard code for no definition in protobuf
			task.Status = common_proto.TaskStatus_UPDATE_SUCCESS
			if err := c.UpdateTask(task.Name, task.Image, 2, 80, 80); err != nil {
				glog.V(1).Infoln(err)
				task.Status = common_proto.TaskStatus_UPDATE_FAILED
				chTask.Report = err.Error()
			}

			send(chTask.stream, &common_proto.Event{
				EventType: common_proto.Operation_TASK_UPDATE,
				OpMessage: &common_proto.Event_TaskFeedback{
					TaskFeedback: &common_proto.TaskFeedback{TaskId: task.Id, Url: task.Url, DataCenter: task.DataCenterId, Report: chTask.Report}}})

		case common_proto.Operation_TASK_CANCEL:
			task.Status = common_proto.TaskStatus_CANCELLED
			if err := c.CancelTask(task.Name); err != nil {
				glog.V(1).Infoln(err)
				task.Status = common_proto.TaskStatus_CANCEL_FAILED
				chTask.Report = err.Error()
			}

			send(chTask.stream, &common_proto.Event{
				EventType: common_proto.Operation_TASK_CANCEL,
				OpMessage: &common_proto.Event_TaskFeedback{
					TaskFeedback: &common_proto.TaskFeedback{TaskId: task.Id, Url: task.Url, DataCenter: task.DataCenterId, Report: chTask.Report}}})
		}
	}
}

func heartBeat(c *task.Client, dcName string, stream grpc_dcmgr.DCStreamer_ServerStreamClient) error {
	dataCenter := common_proto.DataCenter{
		Name: dcName,
	}

	var message = common_proto.Event{
		EventType: common_proto.Operation_HEARTBEAT,
		OpMessage: &common_proto.Event_DataCenter{
			DataCenter: &dataCenter,
		},
	}

	tasks, err := c.ListTask()
	if err != nil {
		glog.V(1).Infoln(err)
		dataCenter.Report = err.Error()
	} else {
		dataCenter.Report = strings.Join(tasks, "\n")
	}

	return send(stream, &message)
}

func dialStream(timeout time.Duration, hubServer string) (grpc_dcmgr.DCStreamer_ServerStreamClient, func(), error) {
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

	client := grpc_dcmgr.NewDCStreamerClient(conn)
	stream, err := client.ServerStream(ctx)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		conn.Close()
		return nil, nil, errors.Wrap(err, "listen k8s task")
	}

	return stream, func() {
		if cancel != nil {
			cancel()
		}
		stream.CloseSend()
		conn.Close()
	}, nil
}

func send(stream grpc_dcmgr.DCStreamer_ServerStreamClient, msg *common_proto.Event) error {
	if err := stream.Send(msg); err != nil {
		glog.V(2).Infof("send (%v) fail: %s", *msg, err)
		return err
	}

	glog.V(3).Infof("send %s success", msg.EventType)
	return nil
}
