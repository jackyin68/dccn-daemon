# Set up CI/CD for Ankr daemon

## Objective

Set up CI/CD pipeline for Ankr daemon using CircleCI so each commit will create the Docker image for the daemon and upload to a Kubernetes cluster after any commit.

## Specifics

To imitate the Ankr daemon, I utilized a "Hello World" program that serves http requests, and print out a simple greeting message. This serves to test the functionality of the Ankr daemon to listen for requests, and in general the functionality of a persistent application. 

The code can be found here: https://gowebexamples.com/hello-world/

After commiting a change to the dccn-daemon repository, CircleCI will automatically create and run a new job, in which a docker image will be built based off of the Dockerfile specified in the same directory, in this case being the hello-world go example. Afterwards, the Docker image will be pushed to Ankr's AWS ECR registry, from which our Kubernetes clusters can pull from.

Afterwards, we can test the Docker image on the Kubernetes cluster, and attest that if we visit the associated hello-world program's ip address, the http request will be served. 

## Requirements

To run this CI/CD pipeline and deploy the Docker image on Kubernetes, the dependencies required are:
* Kubernetes CLI: `kubectl` (https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* AWS CLI: `aws` (https://docs.aws.amazon.com/cli/latest/userguide/installing.html)
* Kubernetes Operations: `kops` (https://github.com/kubernetes/kops)
* git 

## Running CI/CD pipeline

Whenever we make a commit to github for the dccn-daemon repo, CircleCI will automatically create a job that will create a Docker image based on the specifications in the Dockerfile and push it to ECR. In this case, we are creating a hello-world docker image, and to test the pipeline, we simply need to make a trivial change to one of our files in this repository, such as an extra newline. After committing and pushing that trivial change, the pipeline will trigger and a Docker image will be created and pushed to ECR.

1. To test the docker image created by the pipeline on a Kubernetes cluster, we can follow this tutorial here: https://medium.com/containermind/how-to-create-a-kubernetes-cluster-on-aws-in-few-minutes-89dda10354f4 up to step 8. 

2. Then we can run the command to create a cluster, with microinstances for cost consideration.
```
kops create cluster --node-count=2 --master-size=t2.micro --master-volume-size 16 --node-size=t2.micro --node-volume-size 8 --zones=us-west-1a --name=${KOPS_CLUSTER_NAME}
```

Follow the instructions listed by the terminal to officially create and validate the cluster.

3. Kubernetes cluster uses the Secret of docker-registry type to authenticate with a container registry to pull a private image, the access passpord can be obtain through "aws ecr get-authorization-token" command refering the aws cli guide:
https://docs.aws.amazon.com/cli/latest/reference/ecr/get-authorization-token.html

```
$ export password=$(aws ecr get-authorization-token --output text --query authorizationData[].authorizationToken | base64 -D | cut -d: -f2)
```

Create this Secret, naming it aws-ecr:
```
$ kubectl create secret docker-registry aws-ecr --docker-server=https://815280425737.dkr.ecr.us-west-2.amazonaws.com/dccn_ecr --docker-username=AWS --docker-password=$password --docker-email=hanping@ankr.network
```

4. Then create a deployment for this hello world program located in the KubernetesConfigFiles directory:
```
kubectl create -f deployment.yml
```

5. Then we can check the pods running in the Kubernetes cluster:
```
$ kubectl get pod -o wide

NAME                            READY     STATUS    RESTARTS   AGE       IP           NODE
hello-go-dep-85dfc98484-98rgn   1/1       Running   0          1m        100.96.1.4   ip-172-20-45-20.us-west-1.compute.internal
```

Here we can see the deployment is running on node ip-172-20-45-20.us-west-1.compute.internal, then we can check the nodes running in the Kubernetes cluster:

```
$ kubectl get nodes -o wide

NAME                                          STATUS         AGE       VERSION   EXTERNAL-IP      OS-IMAGE                      KERNEL-VERSION
ip-172-20-45-135.us-west-1.compute.internal   Ready,master   9m        v1.10.6   13.57.28.228     Debian GNU/Linux 8 (jessie)   4.4.148-k8s
ip-172-20-45-20.us-west-1.compute.internal    Ready,node     8m        v1.10.6   13.57.244.234    Debian GNU/Linux 8 (jessie)   4.4.148-k8s
ip-172-20-57-72.us-west-1.compute.internal    Ready,node     8m        v1.10.6   54.183.252.174   Debian GNU/Linux 8 (jessie)   4.4.148-k8s
```

Now that we have the nodes up and running, we need to change the security groups for both nodes so that we can access the hello world application. To do so, we go to AWS EC2, find "Security groups" under the instance "Description" and then create a new "Inbound" rule, exposing ports 30000-30600 and on the ip 0.0.0.0. 

Here we can see the external ip for ip-172-20-45-20.us-west-1.compute.internal is 13.57.244.234, and we can navigate to 13.57.244.234:30042 to see the hello world example running.

## Cleanup

To delete the Kubernetes cluster, we can run the command:

```
kops delete cluster --name=${KOPS_CLUSTER_NAME} --state=s3://${KOPS_BUCKET_NAME} --yes
```


