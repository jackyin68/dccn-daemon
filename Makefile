PROTO_FILES	= types/*.proto
LDFLAGS		= -ldflags \
		 "-X main.version=$(shell git rev-parse --abbrev-ref HEAD) \
		 -X main.commit=$(shell git rev-parse --short HEAD) \
		 -X main.date=$(shell date +%Y-%m-%dT%H:%M:%S%z)"

default: govet gofmt gotest build

build:
	go build $(LDFLAGS)

image:
	docker build --rm -t dccn-daemon:latest .

govet:
	go vet ./...

gotest:
	go test -race ./...

gofmt:
	go fmt ./...

golint:
	golangci-lint run

clean:
	rm -f dccn-daemon

test:
	go test ./...

dev-install:
	dep ensure -v
	go get -v golang.org/x/tools/cmd/stringer
	go get -v github.com/gogo/protobuf/protoc-gen-gogo
	go get -v github.com/golang/mock/mockgen
	go get -v github.com/golangci/golangci-lint/cmd/golangci-lint
	# go get -v github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
	# go get -v github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger

gen-dependency:
	go generate -x ./...

	$(eval GOGO := github.com/gogo/protobuf)
	$(eval GOOGLEAPIS := github.com/grpc-ecosystem/grpc-gateway)
	[[ -d vendor/${GOGO} ]] || git clone https://${GOGO}.git vendor/${GOGO}
	[[ -d vendor/${GOOGLEAPIS} ]] || git clone https://${GOOGLEAPIS}.git vendor/${GOOGLEAPIS}

	protoc -I. \
		-Ivendor -Ivendor/${GOGO}/protobuf \
		-Ivendor/${GOOGLEAPIS}/third_party/googleapis \
		--gofast_out=plugins=grpc:. ${PROTO_FILES}
	# protoc -I. \
	# 	-Ivendor -Ivendor/github.com/gogo/protobuf/protobuf \
	# 	-Ivendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
	# 	--grpc-gateway_out=logtostderr=true:. ${PROTO_FILES}
	# protoc -I. \
	# 	-Ivendor -Ivendor/github.com/gogo/protobuf/protobuf \
	# 	-Ivendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
	# 	--swagger_out=logtostderr=true:. ${PROTO_FILES}

.PHONY: default \
	build \
	image \
	govet \
	gotest \
	gofmt \
	golint \
	clean \
	test \
	dev-install \
	gen-dependency
