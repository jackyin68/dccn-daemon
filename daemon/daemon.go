package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	common_proto "github.com/Ankr-network/dccn-common/protos/common"
	grpc_dcmgr "github.com/Ankr-network/dccn-common/protos/dcmgr/v1/grpc"
	"github.com/Ankr-network/dccn-daemon/task"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	//	"google.golang.org/grpc/keepalive"
)

type taskCtx struct {
	*common_proto.DCStream
	stream grpc_dcmgr.DCStreamer_ServerStreamClient
	ctx    context.Context
}

var dataCenterName string
var startTimestamp uint64
var modTimestamp uint64

// ServeTask will serve the task metering with blockchain logic
func ServeTask(cfgpath, namespace, ingressHost, hubServer, dcName,
	tendermintServer, tendermintWsEndpoint string) error {
	dataCenterName = dcName
	startTimestamp = uint64(time.Now().UnixNano())
	tasker, err := task.NewTasker(cfgpath, namespace, ingressHost)
	if err != nil {
		return err
	}

	go taskMetering(tasker, dcName, namespace, tendermintServer, tendermintWsEndpoint)

	var taskCh = make(chan *taskCtx) // block chan, serve single task one time
	go taskOperator(tasker, dcName, taskCh)
	return taskReciver(tasker, hubServer, dcName, taskCh)
}

func taskMetering(t *task.Tasker, dcName, namespace, server, wsEndpoint string) {
	once := &sync.Once{}
	tick := time.Tick(30 * time.Second)
	for range tick {
		metering, err := t.Metering()
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

func taskReciver(t *task.Tasker, hubServer, dcName string, taskCh chan<- *taskCtx) error {
	// try once to test connection, all tests should finish in 5s
	// todo remove such codes has no meaning
	stream, closeStream, err := dialStream(5*time.Second, hubServer)
	if err != nil {
		return err
	}
	closeStream()
	glog.Infoln("Task reciver started.")

	redial := true

	for {
		if redial {
			stream, closeStream, err = dialStream(5*time.Second, hubServer)
			if err != nil {
				glog.Errorln("client fail to receive task:", err)
				time.Sleep(50000 * time.Second)
				continue
			}

			//regist dc  if connection why send heart beat failed ?
			if err := heartBeat(t, dcName, stream); err != nil {
				closeStream()
			} else {
				redial = false
			}

			go startHeartBeatThread(t, dcName, stream, &redial)
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
			glog.V(1).Infof("new task: %v", in)
			taskCh <- &taskCtx{
				DCStream: in,
				stream:   stream,
			}
		}
	}
}

func startHeartBeatThread(t *task.Tasker, dcName string, stream grpc_dcmgr.DCStreamer_ServerStreamClient, redial *bool) {

	for {
		log.Printf("send heart beat\n")
		if err := heartBeat(t, dcName, stream); err != nil {
			log.Printf("send heart beat failed  %v\n", err)
			*redial = true
			return // stream error
		} else {
			log.Printf("send heart beat ok \n")
			time.Sleep(30 * time.Second)
		}
	}
}

func taskOperator(t *task.Tasker, dcName string, taskCh <-chan *taskCtx) {
	glog.Infoln("Task operator started.")
	for {
		chTask, ok := <-taskCh
		if !ok {
			return
		}
		modTimestamp = uint64(time.Now().UnixNano())

		task := chTask.GetTask()
		if task.GetTypeDeployment() == nil && task.GetTypeJob() == nil && task.GetTypeCronJob() == nil {
			glog.Errorln("invalid type data, IGNORE THIS REQUEST")
			continue
		}
		task.DataCenterName = dataCenterName

		var (
			deployment = task.GetTypeDeployment()
			job        = task.GetTypeJob()
			cronjob    = task.GetTypeCronJob()
			attr       = task.GetAttributes()
			err        error
		)
		err = errors.New("")

		switch chTask.OpType {
		case common_proto.DCOperation_TASK_CREATE:
			switch task.Type {
			case common_proto.TaskType_DEPLOYMENT:
				err = t.CreateTasks(task.Id, strings.Split(deployment.Image, ",")...)
			case common_proto.TaskType_JOB:
				err = t.CreateJobs(task.Id, "", job.Image)
			case common_proto.TaskType_CRONJOB:
				err = t.CreateJobs(task.Id, cronjob.Schedule, cronjob.Image)
			default:
				err = errors.Errorf("INVALID TASK TYPE: %s", task.Type)
				glog.Errorln(err)
			}
			if err != nil {
				task.Status = common_proto.TaskStatus_START_FAILED
				glog.V(1).Infoln(err)
			} else {
				task.Status = common_proto.TaskStatus_START_SUCCESS
			}

			log.Printf("create task %+v \n ", task)
			if err == nil {
				err = errors.New("")
			}

			chTask.DCStream.OpPayload = &common_proto.DCStream_TaskReport{
				TaskReport: &common_proto.TaskReport{Task: task, Report: err.Error()}}
			send(chTask.stream, chTask.DCStream)

		case common_proto.DCOperation_TASK_UPDATE:
			glog.V(1).Infof("Operation_TASK_UPDATE  task  %v", task)
			// FIXME: hard code for no definition in protobuf
			task.Status = common_proto.TaskStatus_UPDATE_SUCCESS

			var err error
			switch task.Type {
			case common_proto.TaskType_DEPLOYMENT:
				err = t.UpdateTask(task.Id, deployment.Image, uint32(attr.Replica), 80, 80)
			case common_proto.TaskType_JOB:
				err = t.CreateJobs(task.Id, "", job.Image)
			case common_proto.TaskType_CRONJOB:
				err = t.CreateJobs(task.Id, cronjob.Schedule, cronjob.Image)
			default:
				err = errors.Errorf("INVALID TASK TYPE: %s", task.Type)
				glog.Errorln(err)
			}
			if err != nil {
				glog.V(1).Infoln(err)
				task.Status = common_proto.TaskStatus_UPDATE_FAILED
			} else {
				task.Status = common_proto.TaskStatus_UPDATE_SUCCESS
			}

			if err == nil {
				err = errors.New("")
			}

			chTask.DCStream.OpPayload = &common_proto.DCStream_TaskReport{
				TaskReport: &common_proto.TaskReport{Task: task, Report: err.Error()}}
			send(chTask.stream, chTask.DCStream)

		case common_proto.DCOperation_TASK_CANCEL:
			glog.V(1).Infof("Operation_TASK_CANCEL  task  %v", task)
			task.Status = common_proto.TaskStatus_CANCELLED

			var err error
			switch task.Type {
			case common_proto.TaskType_DEPLOYMENT:
				err = t.CancelTask(task.Id)
			case common_proto.TaskType_JOB:
				err = t.CancelJob(task.Id, "")
			case common_proto.TaskType_CRONJOB:
				err = t.CancelJob(task.Id, "")
			default:
				err = errors.Errorf("INVALID TASK TYPE: %s", task.Type)
				glog.Errorln(err)
			}
			if err != nil {
				glog.V(1).Infoln(err)
				task.Status = common_proto.TaskStatus_CANCEL_FAILED
			} else {
				task.Status = common_proto.TaskStatus_CANCELLED
			}
			if err == nil {
				err = errors.New("")
			}

			chTask.DCStream.OpPayload = &common_proto.DCStream_TaskReport{
				TaskReport: &common_proto.TaskReport{Task: task, Report: err.Error()}}
			send(chTask.stream, chTask.DCStream)
		}
	}
}

func heartBeat(t *task.Tasker, dcName string, stream grpc_dcmgr.DCStreamer_ServerStreamClient) error {
	message := common_proto.DCStream_DataCenter{
		DataCenter: &common_proto.DataCenter{
			Id:     "",
			Name:   dataCenterName,
			Status: common_proto.DCStatus_AVAILABLE,
			DcAttributes: &common_proto.DataCenterAttributes{
				WalletAddress:    "",
				CreationDate:     startTimestamp,
				LastModifiedDate: modTimestamp,
			},
			DcHeartbeatReport: &common_proto.DCHeartbeatReport{
				Metrics:    "",
				Report:     "",
				ReportTime: uint64(time.Now().UnixNano()),
			},
		},
	}

	if metrics, err := t.Metrics(); err != nil {
		glog.V(1).Infoln(err)
	} else {
		data, _ := json.Marshal(metrics)
		message.DataCenter.DcHeartbeatReport.Metrics = string(data)
	}

	if tasks, err := t.ListTask(); err != nil {
		glog.V(1).Infoln(err)
	} else {
		message.DataCenter.DcHeartbeatReport.Report = strings.Join(tasks, "\n")
	}

	return send(stream, &common_proto.DCStream{
		OpType:    common_proto.DCOperation_HEARTBEAT,
		OpPayload: &message,
	})
}

func dialStream(timeout time.Duration, hubServer string) (grpc_dcmgr.DCStreamer_ServerStreamClient, func(), error) {
	var cancel context.CancelFunc
	var ctx = context.Background()
	// if timeout > 0 {
	// 	ctx, cancel = context.WithTimeout(ctx, timeout)
	// }

	conn, err := grpc.DialContext(ctx, hubServer, grpc.WithInsecure())
	// grpc.WithKeepaliveParams(keepalive.ClientParameters{ // TODO: dynamic config in config file
	// 	Time:    20 * time.Second,
	// 	Timeout: 60 * time.Second,
	// }))
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

func send(stream grpc_dcmgr.DCStreamer_ServerStreamClient, msg *common_proto.DCStream) error {
	if err := stream.Send(msg); err != nil {
		glog.V(2).Infof("send (%v) fail: %s", *msg, err)
		return err
	}

	glog.V(3).Infof("send %s success", msg.OpType)
	return nil
}
