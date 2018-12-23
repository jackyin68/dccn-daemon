# UPGRADE: Go Docker image
FROM golang:1.10-alpine3.8

RUN apk update && \
    apk add --no-cache git && \
    apk add --update --no-cache bash && \
    apk add --no-cache openssh
RUN go get github.com/golang/dep/cmd/dep

COPY id_rsa /root/.ssh/
RUN ssh-keyscan github.com >> ~/.ssh/known_hosts
RUN chmod go-w /root
RUN chmod 700 /root/.ssh
RUN chmod 600 /root/.ssh/id_rsa

WORKDIR $GOPATH/src/dccn-daemon
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only
COPY . $GOPATH/src/dccn-daemon

EXPOSE 8080

CMD go run main.go --ip dc-stage.dccn.ankr.network --port 50051 --dcName datacenter_1
