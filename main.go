
package main

import (
	"flag"
	"io"
	"fmt"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"log"
	"time"
	"golang.org/x/net/context"
	gogrpc "google.golang.org/grpc"
	pb "dccn_hub/protocol"
)

const (
        CREATE_TASK = "create a task"
        LIST_TASK = "list a task"
        DELETE_TASK = "delete a task"
)

const (
	address  = "10.0.0.61:50051"
)


// runRouteChat receives a sequence of route notes, while sending notes for various locations.
func sendTaskStatus(client pb.DccncliClient, clientset *kubernetes.Clientset) {
        var taskType string
        ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Second)
        defer cancel()
        stream, err := client.K8Task(ctx)
        if err != nil {
            log.Fatalf("%v.RouteChat(_) = _, %v", client, err)
        }
        waitc := make(chan struct{})
        go func() {
            for {
                in, err := stream.Recv()
                if err == io.EOF {
                        close(waitc)
                        return
                }
                if err != nil {
                        log.Fatalf("Failed to receive a note : %v", err)
                }
                //fmt.Printf("Got message %d %s %s %s\n", in.Taskid , in.Name, in.Extra, in.Type)

                if in.Type == "HeartBeat" {
                    fmt.Printf("Heartbeat!\n")
                } else  {
                    fmt.Printf("Got message %d %s %s %s\n", in.Taskid , in.Name, in.Extra, in.Type)
                }

                if in.Type == "NewTask" {
                    taskType = CREATE_TASK
                }

                if in.Type == "CancelTask" {
                    taskType = DELETE_TASK
                }

                if taskType == CREATE_TASK {
                    fmt.Printf("starting the task\n")
                    ankr_create_task(clientset)
                    fmt.Printf("finish starting the task\n")
                    var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Status:"StartSuccess", Datacenter:"datacenter_2"}
                    if err := stream.Send(&messageSucc); err != nil {
                       log.Fatalf("Failed to send a note: %v", err)
                    }
                }

                if taskType == DELETE_TASK {
                    fmt.Printf("canceling the task")
                    ankr_delete_task(clientset)
                    fmt.Printf("finish canceling the task")
                    var messageSucc = pb.K8SMessage{Taskid: in.Taskid, Status:"Cancelled", Datacenter:"datacenter_2"}
                    if err := stream.Send(&messageSucc); err != nil {
                       log.Fatalf("Failed to send a note: %v", err)
                    }
                }
              
                taskType = ""
            }
        }()

        //var messageFail = pb.TaskStatus{Taskid: -1, Status:"Failure"}
        var messageSucc = pb.K8SMessage{Taskid:  1, Status:"StartSuccess", Datacenter:"datacenter_2"}
        if err := stream.Send(&messageSucc); err != nil {
            log.Fatalf("Failed to send a note: %v", err)
        }

        fmt.Printf("send TaskStatus  message %d %s \n", messageSucc.Taskid , messageSucc.Status)

        //stream.CloseSend()
        <-waitc
}


func querytask(clientset *kubernetes.Clientset) {
	conn, err := gogrpc.Dial(address, gogrpc.WithInsecure())
	if err != nil {
	    log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewDccncliClient(conn)

        for  {
            sendTaskStatus(c, clientset)
        }

/*synchronous one time call*/
/*
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second )
	defer cancel()

        r, err := c.K8QueryTask(ctx, &pb.QueryTaskRequest{Name:"datacenter_2"})
        if err != nil {
            fmt.Printf("Fail to connect to server. Error:\n") 
            log.Fatalf("Client: could not send: %v", err)
        }

        fmt.Printf("received new task  : %d %s %s \n", r.Taskid, r.Name, r.Extra)
*/
}

func sendreport() {
	conn, err := gogrpc.Dial(address, gogrpc.WithInsecure())
	if err != nil {
	    log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewDccncliClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second )
	defer cancel()
	r, err := c.K8ReportStatus(ctx, &pb.ReportRequest{Name:"datacenter_2",Report:"job2 job2 job3 host 100", Host:"127.0.0.67", Port:5009 })
	if err != nil {
            fmt.Printf("Fail to connect to server. Error:\n") 
	    log.Fatalf("Client: could not send: %v", err)
	}

	fmt.Printf("received Status : %s \n", r.Status)
}

func ankr_delete_task(clientset *kubernetes.Clientset) bool {
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	fmt.Println("Deleting deployment...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete("demo-deployment", &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		//panic(err)
                return false
	}

        return true
}
func ankr_list_task(clientset *kubernetes.Clientset) bool {
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	fmt.Printf("Listing deployments in namespace %q:\n", apiv1.NamespaceDefault)
	list, err := deploymentsClient.List(metav1.ListOptions{})
	if err != nil {
		panic(err)
                return false
	}

	for _, d := range list.Items {
                _, err := deploymentsClient.Get(d.Name, metav1.GetOptions{})
                if err == nil {
                    //fmt.Printf("status.AvailableReplicas:%s\n", d.Status.AvailableReplicas)
                    //fmt.Printf("revision:%s\n", d.Revision)
                    fmt.Printf("image:%s\n", d.Spec.Template.Spec.Containers[0].Image)
                    //fmt.Printf("NodeName:%s\n", d.Spec.Template.Spec.NodeName)
                    //fmt.Printf("Hostname:%s\n", d.Spec.Template.Spec.Hostname)
                    //fmt.Printf("containers:%s\n", d.Spec.Template.Spec.Containers[0])
                    //fmt.Printf("%s\n", cc)
                }
		fmt.Printf("task name: %s (%d replicas running)\n\n", d.Name, *d.Spec.Replicas)
	}

        return true
}

func ankr_create_task(clientset *kubernetes.Clientset) bool {
	deploymentsClient := clientset.AppsV1().Deployments(apiv1.NamespaceDefault)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "demo-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "demo",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "demo",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "web",
							Image: "nginx:1.12",
							Ports: []apiv1.ContainerPort{
								{
									Name:          "http",
									Protocol:      apiv1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}

	fmt.Println("Creating deployment...")
	result, err := deploymentsClient.Create(deployment)
	if err != nil {
		//panic(err)  //probably already exist
                fmt.Println("probably already exist.\n")
                return false
	}
	fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())

        return true
}

func int32Ptr(i int32) *int32 { return &i }

func main() {
        var taskType string
        pboolCreate := flag.Bool("create", false, "create task")
        pboolList := flag.Bool("list", false, "list task")
        pboolDelete := flag.Bool("delete", false, "delete task")

	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), 
                             "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()

        if *pboolCreate {
            taskType = CREATE_TASK
        } else if *pboolList {
            taskType = LIST_TASK
        } else if *pboolDelete {
            taskType = DELETE_TASK
        }

        fmt.Println(taskType)

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

        switch taskType {
            case CREATE_TASK:
                ankr_create_task(clientset)
            case LIST_TASK:
                ankr_list_task(clientset)
            case DELETE_TASK:
                ankr_delete_task(clientset)
        }

        querytask(clientset)
}
