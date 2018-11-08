# UPGRADE: Go Docker image
FROM golang:1.11.1-stretch

WORKDIR /go/src/hello-world
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

EXPOSE 8080

CMD ["hello-world"]
