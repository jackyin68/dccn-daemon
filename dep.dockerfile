# UPGRADE: Go Docker image
FROM golang:1.10-alpine3.8

RUN apk update && \
    apk add --no-cache git && \
    apk add --update --no-cache bash && \
    apk add --no-cache openssh
RUN go get github.com/golang/dep/cmd/dep

WORKDIR $GOPATH/src/dccn-daemon
COPY . $GOPATH/src/dccn-daemon

EXPOSE 8080

CMD go run main.go \
    --ip hub.ankr.network \
    --port 50051 \
    --dcName ankr_datacenter1 \
    --kubeconfig ankr.yaml
