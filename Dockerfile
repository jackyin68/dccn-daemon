# Compile
FROM golang:1.11-alpine AS compiler

RUN apk add --no-cache git

# not in GOPATH, go modules auto enabled
WORKDIR /dccn-daemon
COPY . .

RUN CGO_ENABLED=0 go vet && CGO_ENABLED=0 go test -v
RUN CGO_ENABLED=0 go build -v -ldflags="-s -w \
    -X main.version=$(git rev-parse --abbrev-ref HEAD) \
    -X main.commit=$(git rev-parse --short HEAD) \
    -X main.date=$(date +%Y-%m-%dT%H:%M:%S%z)"


# Build image, alpine provides more possibilities than scratch
FROM alpine

COPY --from=compiler /dccn-daemon/dccn-daemon /dccn-daemon
RUN ln -s /dccn-daemon /usr/local/bin/dccn-daemon

ARG URL_BRANCH
ENV URL_BRANCH=${URL_BRANCH}

CMD ["dccn-daemon","version"]
