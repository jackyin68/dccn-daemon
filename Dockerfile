# UPGRADE: Go Docker image
FROM golang:alpine

RUN apk update && apk add git && apk add --update bash && apk add openssh
RUN go get github.com/golang/dep/cmd/dep

COPY id_rsa /root/.ssh/
RUN ssh-keyscan github.com >> ~/.ssh/known_hosts
RUN chmod go-w /root
RUN chmod 700 /root/.ssh
RUN chmod 600 /root/.ssh/id_rsa


WORKDIR $GOPATH/src/dccn-daemon
COPY main.go $GOPATH/src/dccn-daemon
COPY ankr.conf $GOPATH/src/dccn-daemon

COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only

RUN go get github.com/golang/dep/cmd/dep
RUN go get k8s.io/client-go/kubernetes
RUN go get k8s.io/client-go/tools/clientcmd
RUN go get k8s.io/client-go/util/homedir
RUN go get k8s.io/client-go/util/retry
RUN go get google.golang.org/grpc
RUN go get golang.org/x/net/context
RUN go get k8s.io/apimachinery/pkg/apis/meta/v1
RUN go get k8s.io/api/core/v1
RUN go get k8s.io/api/apps/v1
RUN dep ensure -vendor-only

EXPOSE 8080

CMD go run main.go --ip hub.ankr.network --port 50051 --dcName ankr_datacenter1 --kubeconfig ankr.conf
