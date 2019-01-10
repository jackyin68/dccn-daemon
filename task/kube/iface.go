package kube

import (
	"bufio"
	"context"
	"io"

	"github.com/Ankr-network/dccn-daemon/types"
)

//go:generate mockgen -package $GOPACKAGE -destination mock_client.go github.com/Ankr-network/dccn-daemon/task/kube Client
type Client interface {
	Deploy(manifest *types.ManifestGroup) error
	Job(manifest *types.ManifestGroup) error
	CronJob(manifest *types.ManifestGroup) error
	TeardownNamespace() error

	ListDeployments() (names, contents []string, err error)
	ServiceStatus(name string) (*types.ServiceStatusResponse, error)
	ServiceLogs(ctx context.Context, tailLines int64, follow bool) ([]*ServiceLog, error)

	Inventory() (nodes []Node, err error)
	Metering() (map[string]*types.ResourceUnit, error)
}

// ServiceLog definition
type ServiceLog struct {
	Name    string
	Stream  io.ReadCloser
	Scanner *bufio.Scanner
}

func newServiceLog(name string, stream io.ReadCloser) *ServiceLog {
	return &ServiceLog{
		Name:    name,
		Stream:  stream,
		Scanner: bufio.NewScanner(stream),
	}
}

// Node definition
type Node interface {
	ID() string
	Available() types.ResourceUnit
}

type node struct {
	id        string
	available types.ResourceUnit
}

func newNode(id string, available types.ResourceUnit) Node {
	return &node{id: id, available: available}
}

func (n *node) ID() string {
	return n.id
}

func (n *node) Available() types.ResourceUnit {
	return n.available
}
