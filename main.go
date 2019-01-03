package main

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	ankr_const "github.com/Ankr-network/dccn-common"
	pb "github.com/Ankr-network/dccn-common/protocol/k8s"
	"golang.org/x/net/context"
	gogrpc "google.golang.org/grpc"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

const (
	CREATE_TASK = "create a task"
	LIST_TASK   = "list a task"
	DELETE_TASK = "delete a task"
	UPDATE_TASK = "update task"
)

const (
	ADDRESS     = "10.0.0.61:50051"
	MAX_REPLICA = 20
)

var gAddressCLI = ""
var gDcNameCLI = ""
var gTotalPodNum = 0

// runRouteChat receives a sequence of route notes, while sending notes for various locations.
func sendTaskStatus(client pb.Dccnk8SClient, clientset *kubernetes.Clientset) int {
	var ret int = 0
	var taskType string
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
	defer cancel()
	stream, err := client.K8Task(ctx)
	if err != nil {
		return 3
	}
	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				time.Sleep(3 * time.Second)
				taskType = ""
				continue
			}
			if err != nil {
				fmt.Println(err)
				fmt.Printf("Failed to receive a note : %v \n", err)
				ret = 1
				return
			}

			if in.Type == "HeartBeat" {
				fmt.Printf("Heartbeat!\n")
			} else {
				fmt.Printf("Got message %d %d %s %s %s %s %s\n", in.Taskid, in.Replica, in.Name, in.Image, in.Extra, in.Type, in.TaskType)

				if in.Type == "NewTask" {
					taskType = CREATE_TASK
				} else if in.Type == "CancelTask" {
					taskType = DELETE_TASK
				} else if in.Type == "UpdateTask" {
					taskType = UPDATE_TASK
				} else {
					fmt.Println("Unknown task type:", in.Type)
				}
			}

			if taskType == CREATE_TASK {
				fmt.Printf("starting the task\n")
				ret := ankr_create_task(clientset, in.Name, in.Image)
				if ret {
					podNumNew := 0
					podsClient, err := clientset.CoreV1().Pods(apiv1.NamespaceDefault).List(metav1.ListOptions{})
					if err != nil {
						return
					}

					for _, pod := range podsClient.Items {
						if pod.Status.Phase != "Running" && !pod.Status.ContainerStatuses[0].Ready {
							fmt.Println(pod.Name, " not running.")
							continue
						}
						podNumNew += 1
					}

					fmt.Printf("total tasks %d; after creating total tasks %d\n", gTotalPodNum, podNumNew)
					if gTotalPodNum > podNumNew {
						fmt.Println("remove the failed task.")
						ankr_delete_task(clientset, in.Name)
						fmt.Printf("fail to start the task\n")
						var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "StartFailure", Datacenter: gDcNameCLI}
						if err := stream.Send(&messageSucc); err != nil {
							fmt.Printf("Failed to send a note: %v\n", err)
						}
						taskType = ""
						continue
					} else {
						gTotalPodNum = podNumNew
					}

					fmt.Printf("finish starting the task\n")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "StartSuccess", Datacenter: gDcNameCLI, Url: "ankr.com"}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}

				} else {
					fmt.Printf("fail to start the task\n")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "StartFailure", Datacenter: gDcNameCLI}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}
				}
			}

			if taskType == DELETE_TASK {
				fmt.Printf("canceling the task")
				ret := ankr_delete_task(clientset, in.Name)
				if !ret {
					fmt.Printf("fail to cancel the task")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "CancelFailure", Datacenter: gDcNameCLI}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}
				} else {
					fmt.Printf("finish canceling the task")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "Cancelled", Datacenter: gDcNameCLI}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}
				}
			}

			if taskType == UPDATE_TASK {
				fmt.Printf("updating the replica/image")
				ret := ankr_update_task(clientset, int32(in.Replica), in.Name, in.Image)
				if !ret {
					fmt.Printf("fail to update the task")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "UpdateFailure", Datacenter: gDcNameCLI}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}
				} else {
					fmt.Printf("finish updating the task")
					var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Taskname: in.Name, Status: "UpdateSuccess", Datacenter: gDcNameCLI}
					if err := stream.Send(&messageSucc); err != nil {
						fmt.Printf("Failed to send a note: %v\n", err)
					}
				}
			}

			taskType = ""
		}
	}()

	for {
		var messageSucc = pb.K8SMessage{Datacenter: gDcNameCLI, Taskname: "", Type: "HeartBeat", Report: ankr_list_task(clientset)}
		if err := stream.Send(&messageSucc); err != nil {
			fmt.Printf("Failed to send a note: %v \n", err)
			ret = 2
			return ret
		} else {
			fmt.Printf("Send message to Hub, %s \n", messageSucc.Type)
		}

		time.Sleep(ankr_const.HeartBeatInterval * time.Second)
	}

	// <-waitc

	// return 0
}

func querytask(clientset *kubernetes.Clientset) int {
	var hubAddress string = gAddressCLI
	if len(hubAddress) == 0 {
		hubAddress = ADDRESS
	}

	conn, err := gogrpc.Dial(hubAddress, gogrpc.WithInsecure())
	if err != nil {
		//log.Fatalf("did not connect: %v", err)
		// To do: define better error code or error handling
		return 1
	}
	defer conn.Close()
	c := pb.NewDccnk8SClient(conn)

	return sendTaskStatus(c, clientset)
}

func ankr_delete_task(clientset *kubernetes.Clientset, dockerName string) bool {
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	fmt.Println("Deleting deployment...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(dockerName, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		fmt.Println(err)
		return false
	}

	return true
}

func ankr_update_task(clientset *kubernetes.Clientset, num int32, taskname string, image string) bool {
	if (num == 0) && (len(image) == 0) {
		return false
	}

	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		result, getErr := deploymentsClient.Get(taskname, metav1.GetOptions{})
		if getErr != nil {
			fmt.Printf("Failed to get latest version of Deployment: %v\n", getErr)
			return getErr
		}
		if num != 0 {
			result.Spec.Replicas = int32Ptr(num)
		}
		if len(image) != 0 {
			result.Spec.Template.Spec.Containers[0].Image = image //"nginx:1.13"
		}
		_, updateErr := deploymentsClient.Update(result)
		if updateErr != nil {
			fmt.Printf("Failed to update task: %v\n", updateErr)
		}
		return updateErr
	})

	if retryErr != nil {
		fmt.Printf("Update failed: %v\n", retryErr)
		return false
	}

	return true
}

func ankr_list_task(clientset *kubernetes.Clientset) string {
	result := ""
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)
	list, err := deploymentsClient.List(metav1.ListOptions{})
	if err != nil {
		fmt.Println(err)
		fmt.Printf("Probabaly the kubenetes(minikube) not started.\n")
		return ""
	}

	for _, d := range list.Items {
		_, err := deploymentsClient.Get(d.Name, metav1.GetOptions{})
		fmt.Printf("task name: %s, image:%s (%d replicas running)\n\n", d.Name,
			d.Spec.Template.Spec.Containers[0].Image, *d.Spec.Replicas)
		result += "Task:" + string(d.Name) + "," + "Image:" + d.Spec.Template.Spec.Containers[0].Image +
			"Replicas:" + strconv.Itoa(int(*d.Spec.Replicas)) + "\n"
	}

	return result
}

func ankr_create_task(clientset *kubernetes.Clientset, dockerName string, dockerImage string) bool {
	var containerList []apiv1.Container
	if strings.ContainsAny(dockerImage, ",") {
		imageList := strings.Split(dockerImage, ",")
		containerList = []apiv1.Container{
			{
				Name: string(
					dockerName,
				),
				Image: string(
					imageList[0],
				),
				Ports: []apiv1.ContainerPort{
					{
						Name:          "http",
						Protocol:      apiv1.ProtocolTCP,
						ContainerPort: 80,
					},
				},
			},
			{
				Name: string(
					dockerName + "-2",
				),
				Image: string(
					imageList[1],
				),
				Ports: []apiv1.ContainerPort{
					{
						Name:          "http",
						Protocol:      apiv1.ProtocolTCP,
						ContainerPort: 27017,
					},
				},
			},
		}

	} else {
		containerList = []apiv1.Container{
			{
				Name: string(
					dockerName,
				),
				Image: string(
					dockerImage,
				),
				Ports: []apiv1.ContainerPort{
					{
						Name:          "http",
						Protocol:      apiv1.ProtocolTCP,
						ContainerPort: 80,
					},
				},
			},
		}
	}
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(
				dockerName,
			),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "ankr",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "ankr",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: containerList,
				},
			},
		},
	}

	fmt.Println("Creating deployment...")

	podsClient, err := clientset.CoreV1().Pods(apiv1.NamespaceDefault).List(metav1.ListOptions{})
	fmt.Println(err)
	if err != nil {
		return false
	}

	gTotalPodNum = 0
	for _, pod := range podsClient.Items {
		gTotalPodNum += 1
		fmt.Println(pod.Name, pod.Status.PodIP)
	}
	result, err := deploymentsClient.Create(deployment)
	fmt.Println(err)
	if err != nil {
		fmt.Println("probably already exist:.\n", err, result)
		return false
	}

	time.Sleep(2 * time.Second)

	return true
}

func int32Ptr(i int32) *int32 { return &i }

func main() {
	var taskType string
	var ipCLI string
	var portCLI string
	pboolCreate := flag.Bool("create", false, "create task")
	pboolList := flag.Bool("list", false, "list task")
	pboolDelete := flag.Bool("delete", false, "delete task")

	flag.StringVar(&ipCLI, "ip", "", "ankr hub ip address")
	flag.StringVar(&portCLI, "port", "", "ankr hub port number")
	flag.StringVar(&gDcNameCLI, "dcName", "", "data center name")
	pintReplica := flag.Int("update", 0, "replica number")
	flag.Parse()

	if len(gDcNameCLI) == 0 {
		gDcNameCLI = "datacenter_2"
	}
	if len(ipCLI) != 0 && len(portCLI) != 0 {
		// TODO: verify ip and port input
		gAddressCLI = ipCLI + ":" + portCLI
		fmt.Println(gAddressCLI)
	}

	if *pboolCreate {
		taskType = CREATE_TASK
	} else if *pboolList {
		taskType = LIST_TASK
	} else if *pboolDelete {
		taskType = DELETE_TASK
	} else if *pintReplica != 0 {
		// To do: this should be based on the action instead of Replica number
		taskType = UPDATE_TASK
	}

	if *pintReplica < 0 {
		fmt.Printf("invalid replica number:%d\n", *pintReplica)
		return
	} else if *pintReplica > MAX_REPLICA {
		fmt.Printf("replica number %d it too big. Maximum is %d.\n", *pintReplica, MAX_REPLICA)
		return
	}

	fmt.Println(taskType)
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("Error obtaining configuration of cluster\n")
		panic(err.Error())
	}

	fmt.Println(config.Host)
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error obtaining clientset\n")
		panic(err.Error())
	}

	switch taskType {
	case CREATE_TASK:
		// command line test
		ankr_create_task(clientset, "demo-deployment", "nginx:1.12")
		return
	case LIST_TASK:
		fmt.Printf("%s", ankr_list_task(clientset))
		return
	case DELETE_TASK:
		ankr_delete_task(clientset, "demo-deployment")
		return
	case UPDATE_TASK:
		fmt.Printf("update to %d replica\n", *pintReplica)
		// command line test
		ankr_update_task(clientset, int32(*pintReplica), "demo-deployment", "nginx:1.13")
		return
	}

	for {
		ret := querytask(clientset)
		if ret != 0 {
			time.Sleep(3 * time.Second)
			fmt.Println("Reconnect.")
			continue
		} else {
			fmt.Println("Bye.")
			break
		}
	}
}
