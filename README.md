# Set up CI/CD for Ankr daemon

## Objective

Set up CI/CD pipeline for Ankr daemon using CircleCI, so each commit pushed to GitHub will create the Docker image for the daemon, and push it to the AWS ECR registry.
DevOps engineers can then pull the image from the registry and deploy the application to a Kubernetes cluster.

## Specifics

As a placeholder for the Ankr daemon to be developed in a related issue, we use a simple Go application that serves HTTP requests, and responds with a simple greeting message.
This serves to test the functionality of the Ankr daemon to listen for requests, and in general the functionality of a persistent application.

The code can be found here: https://gowebexamples.com/hello-world/

After committing a change to the `dccn-daemon` repository, CircleCI will automatically create and run a new job, in which a Docker image will be built based off of the `Dockerfile` specified in the same directory, in this case being the `hello-world` Go example.
Afterwards, the Docker image will be pushed to Ankr's AWS ECR registry, from which our Kubernetes clusters can pull from.

After deployment to the Kubernetes cluster, we can test the application by visiting the IP address at which it is exposed.

## Requirements

To run this CI/CD pipeline and deploy the Docker image on Kubernetes, the dependencies required are:
* Kubernetes CLI: `kubectl` (https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* AWS CLI: `aws` (https://docs.aws.amazon.com/cli/latest/userguide/installing.html)
* Kubernetes Operations: `kops` (https://github.com/kubernetes/kops)
* git

## Running CI/CD pipeline

Whenever we make a commit to GitHub for the `dccn-daemon` repo, CircleCI will automatically create a job that will create a Docker image based on the specifications in the Dockerfile and push it to ECR.
In this case, we are creating a hello-world Docker image, and to test the pipeline, we simply need to make a trivial change to one of our files in this repository, such as an extra newline.
After committing and pushing that trivial change, the pipeline will trigger and a Docker image will be created and pushed to ECR.

### Deployment to Kubernetes

1. To test the Docker image created by the pipeline on a Kubernetes cluster, follow this tutorial here: https://medium.com/containermind/how-to-create-a-kubernetes-cluster-on-aws-in-few-minutes-89dda10354f4 up to step 8.

2. Then create a cluster (feat. micro-instances for cost consideration) w/:
```
kops create cluster \\
  --node-count=2 \\
  --master-size=t2.micro \\
  --master-volume-size 16 \\
  --node-size=t2.micro \\
  --node-volume-size 8 \\
  --zones=us-west-1a \\
  --name=${KOPS_CLUSTER_NAME}
```

Follow the instructions listed by the terminal to officially create and validate the cluster.

3. Reetrieve an authorization token from AWS ECR (https://docs.aws.amazon.com/cli/latest/reference/ecr/get-authorization-token.html), so that the Kubernetes cluster can pull the Docker image for the deployment:
```
$ export password=$(aws ecr get-authorization-token --output text --query authorizationData[].authorizationToken | base64 -D | cut -d: -f2)
```

3. Store the authorization token from AWS ECR into a secret in Kubernetes:
```
$ kubectl create secret docker-registry aws-ecr --docker-server=https://815280425737.dkr.ecr.us-west-2.amazonaws.com/dccn_ecr --docker-username=AWS --docker-password=$password --docker-email=hanping@ankr.network
```

4. Deploy the application to Kubernetes using the configuration file in the `KubernetesConfigFiles` directory:
```
kubectl create -f deployment.yml
```

5. Confirms that at least one Pod for this deployment is running in the Kubernetes cluster:
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

### Expose the application to the Internet

Now that we have the nodes up and running, we need to change the security groups for both nodes so that we can access the hello world application.

Go to AWS EC2, find "Security groups" under the instance "Description" and then create a new "Inbound" rule, exposing ports `30000-30600` to all IP addresses (`0.0.0.0`).

In this specific cluster, the external IP address for `ip-172-20-45-20.us-west-1.compute.internal` is `13.57.244.234`, and we can navigate to `http://13.57.244.234:30042` to access the Web application.

## Cleanup

Delete the Kubernetes cluster w/:
```
kops delete cluster \\
  --name=${KOPS_CLUSTER_NAME} \\
  --state=s3://${KOPS_BUCKET_NAME} --yes
```
