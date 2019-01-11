# Compile
FROM golang:1.11-alpine AS compiler

RUN apk add --no-cache git dep openssh-client

WORKDIR /go/src/github.com/Ankr-network/dccn-daemon
COPY . .

# for ci runner, copy ssh private key
COPY id_rsa /root/.ssh/id_rsa
RUN ssh-keyscan github.com >> /root/.ssh/known_hosts \
    && chmod go-w /root \
    && chmod 700 /root/.ssh \
    && chmod 600 /root/.ssh/id_rsa \
    && dep ensure -v -vendor-only

RUN go install -v -ldflags="-s -w \
    -X main.version=$(git rev-parse --abbrev-ref HEAD) \
    -X main.commit=$(git rev-parse --short HEAD) \
    -X main.date=$(date +%Y-%m-%dT%H:%M:%S%z)"


# Build image, alpine offers more possibilities than scratch
FROM alpine

COPY --from=compiler /go/bin/dccn-daemon /usr/local/bin/dccn-daemon
RUN ln -s /usr/local/bin/dccn-daemon /dccn-daemon

CMD ["dccn-daemon","version"]
