# UPGRADE: Go Docker image
# To do: use https://circleci.com/gh/Ankr-network/dccn-daemon/edit#ssh for ssh key in circleci
# To do: use multi-stage build
FROM golang:1.10-alpine3.8

ARG URL_BRANCH
ENV URL_BRANCH ${URL_BRANCH}
RUN apk update && \
    apk add --no-cache git && \
    apk add --update --no-cache bash && \
    apk add --no-cache openssh
# To do: use stable version
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

# To do: the dc name should be auto generated
CMD go run main.go \
    --ip $URL_BRANCH \
    --port 50051 \
    --dcName datacenter_1
