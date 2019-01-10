# Compile
FROM golang:1.11-alpine AS compiler

RUN apk add --no-cache git dep openssh-client

WORKDIR /go/src/github.com/Ankr-network/dccn-daemon
COPY . .

# for ci runner, copy ssh private key
RUN dep ensure -v -vendor-only

RUN go install -v -ldflags="-s -w \
    -X main.version=$(git rev-parse --abbrev-ref HEAD) \
    -X main.commit=$(git rev-parse --short HEAD) \
    -X main.date=$(date +%Y-%m-%dT%H:%M:%S%z)"


# Build image, alpine provide more possibilities than scratch
FROM alpine

COPY --from=compiler /go/bin/dccn-daemon /usr/local/bin/dccn-daemon
RUN ln -s /usr/local/bin/dccn-daemon /dccn-daemon

CMD ["dccn-daemon","version"]
