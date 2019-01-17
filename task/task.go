package task

import (
	"encoding/json"
	"strconv"

	"github.com/Ankr-network/dccn-daemon/task/kube"
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func (t *Tasker) CreateTasks(name string, images ...string) error {
	kubes := []kube.Kube{kube.NewPrepare(t.ns, &types.ManifestService{Name: name})}

	count := len(images)
	switch count {
	case 0:
		return errors.New("no image")
	case 1:
		service := types.NewManifestService(name, images[0])
		kubes = append(kubes, kube.NewDeployment(t.ns, service))
	default:
		for i := range images {
			nameI := name + "-" + strconv.Itoa(i)
			service := types.NewManifestService(nameI, images[i])
			kubes = append(kubes, kube.NewDeployment(t.ns, service))
		}
	}

	return t.run(kubes)
}

func (t *Tasker) CreateJobs(name, crontab string, images ...string) error {
	kubes := []kube.Kube{kube.NewPrepare(t.ns, &types.ManifestService{Name: name})}

	count := len(images)
	switch count {
	case 0:
		return errors.New("no image")
	case 1:
		service := types.NewManifestService(name, images[0])
		if crontab == "" {
			kubes = append(kubes, kube.NewJob(t.ns, service))
		} else {
			kubes = append(kubes, kube.NewCronJob(t.ns, service, crontab))
		}
	default:
		for i := range images {
			nameI := name + "-" + strconv.Itoa(i)
			service := types.NewManifestService(nameI, images[i])
			if crontab == "" {
				kubes = append(kubes, kube.NewJob(t.ns, service))
			} else {
				kubes = append(kubes, kube.NewCronJob(t.ns, service, crontab))
			}
		}
	}

	return t.run(kubes)
}

func (t *Tasker) UpdateTask(name, image string, replicas, internalPort, externalPort uint32) error {
	kubes := []kube.Kube{kube.NewPrepare(t.ns, &types.ManifestService{Name: name})}

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

	kubes = append(kubes, kube.NewDeployment(t.ns, service))
	kubes = append(kubes, kube.NewService(t.ns, service, service.Expose[0]))
	kubes = append(kubes, kube.NewIngress(t.ns, service, service.Expose[0]))
	return t.run(kubes)
}

func (t *Tasker) CancelTask(name string) error {
	service := types.NewManifestService(name, "")
	service.Count = 0
	expose := &types.ManifestServiceExpose{}

	if err := kube.NewDeployment(t.ns, service).Delete(t.client); err != nil {
		return err
	}
	if err := kube.NewService(t.ns, service, expose).Delete(t.client); err != nil {
		return err
	}
	if err := kube.NewIngress(t.ns, service, expose).Delete(t.client); err != nil {
		return err
	}
	return nil
}

func (t *Tasker) ListTask() ([]string, error) {
	res := &appsv1.DeploymentList{}
	if err := kube.NewDeployment(t.ns, &types.ManifestService{}).List(t.client, res); err != nil {
		return nil, err
	}
	if len(res.Items) == 0 {
		return nil, errors.New("no deployment")
	}

	contents := make([]string, 0, len(res.Items))
	for _, item := range res.Items {
		content, _ := json.Marshal(item)
		contents = append(contents, string(content))
	}
	return contents, nil
}

func (t *Tasker) Metering() (map[string]*types.ResourceUnit, error) {
	result := &map[string]*types.ResourceUnit{}
	if err := kube.NewMetering(t.ns, &types.ManifestService{}).List(t.client, result); err != nil {
		return nil, err
	}
	return *result, nil
}

type Metrics struct {
	TotalCPU     int64
	UsedCPU      int64
	TotalMemory  int64
	UsedMemory   int64
	TotalStorage int64
	UsedStorage  int64

	ImageCount    int64
	EndPointCount int64
	NetworkIO     int64 // No data
}

func (t *Tasker) Metrics() (*Metrics, error) {
	result := &kube.Metrics{}
	if err := kube.NewMetrics(t.ns, &types.ManifestService{}).List(t.client, result); err != nil {
		return nil, err
	}

	metrics := &Metrics{}
	for _, metric := range result.NodeTotal {
		metrics.TotalCPU += metric.CPU
		metrics.TotalMemory += metric.Memory
		metrics.TotalStorage += metric.EphemeralStorage
	}
	for _, metric := range result.NodeInUse {
		metrics.UsedCPU += metric.CPU
		metrics.UsedMemory += metric.Memory
		metrics.UsedStorage += metric.EphemeralStorage
	}
	for _, images := range result.NodeImages {
		metrics.ImageCount += int64(len(images))
	}
	for _, count := range result.Endpoints {
		metrics.EndPointCount += count
	}

	return metrics, nil
}
