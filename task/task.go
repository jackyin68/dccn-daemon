package task

import (
	"strconv"

	"github.com/Ankr-network/dccn-daemon/task/kube"
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

type Runner struct {
	client kube.Client
}

func NewRunner(cfgpath, namespace, ingressHost string) (*Runner, error) {
	client, err := kube.NewClient(cfgpath, namespace, ingressHost)
	if err != nil {
		return nil, errors.Wrap(err, "new runner")
	}
	return &Runner{client: client}, nil
}

func (r *Runner) CreateTasks(name string, images ...string) error {
	group := &types.ManifestGroup{}

	count := len(images)
	switch count {
	case 0:
		return errors.New("no image")
	case 1:
		group.Services = []*types.ManifestService{types.NewManifestService(name, images[0])}
	default:
		group.Services = make([]*types.ManifestService, 0, count)
		for i := range images {
			nameI := name + "-" + strconv.Itoa(i)
			group.Services = append(group.Services, types.NewManifestService(nameI, images[i]))
		}
	}

	return r.client.Deploy(group)
}
func (r *Runner) UpdateTask(name, image string, replicas, internalPort, externalPort uint32) error {
	group := &types.ManifestGroup{}
	service := types.NewManifestService(name, image)
	service.Count = replicas
	if internalPort != 0 && externalPort != 0 {
		service.Expose = []*types.ManifestServiceExpose{&types.ManifestServiceExpose{
			Port:         internalPort,
			ExternalPort: externalPort,
			Proto:        string(corev1.ProtocolTCP),
			Service:      name,
			Global:       true,
			Hosts:        []string{},
		}}
	}
	group.Services = []*types.ManifestService{service}

	return r.client.Deploy(group)
}

func (r *Runner) CancelTask(name string) error {
	group := &types.ManifestGroup{}
	service := types.NewManifestService(name, "")
	service.Count = 0
	service.Expose = []*types.ManifestServiceExpose{}
	group.Services = []*types.ManifestService{service}

	return r.client.Deploy(group)
}

func (r *Runner) ListTask() ([]string, error) {
	_, contents, err := r.client.ListDeployments()
	return contents, err
}

func (r *Runner) Metering() (map[string]*types.ResourceUnit, error) {
	return r.client.Metering()
}
