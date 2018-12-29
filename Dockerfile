# UPGRADE: Go Docker image
FROM golang:alpine
ARG URL_BRANCH
RUN apk update && apk add git && apk add --update bash && apk add openssh
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

CMD go run main.go --ip $URL_BRANCH --port 50051 --dcName datacenter_1
