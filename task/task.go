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

func (c *Client) CreateTasks(name string, images ...string) error {
	kubes := []kube.Kube{kube.NewPrepare(c.ns, &types.ManifestService{Name: name})}

	count := len(images)
	switch count {
	case 0:
		return errors.New("no image")
	case 1:
		service := types.NewManifestService(name, images[0])
		kubes = append(kubes, kube.NewDeployment(c.ns, service))
	default:
		for i := range images {
			nameI := name + "-" + strconv.Itoa(i)
			service := types.NewManifestService(nameI, images[i])
			kubes = append(kubes, kube.NewDeployment(c.ns, service))
		}
	}

	return c.run(kubes)
}

func (c *Client) CreateJobs(name, crontab string, images ...string) error {
	kubes := []kube.Kube{kube.NewPrepare(c.ns, &types.ManifestService{Name: name})}

	count := len(images)
	switch count {
	case 0:
		return errors.New("no image")
	case 1:
		service := types.NewManifestService(name, images[0])
		if crontab == "" {
			kubes = append(kubes, kube.NewJob(c.ns, service))
		} else {
			kubes = append(kubes, kube.NewCronJob(c.ns, service, crontab))
		}
	default:
		for i := range images {
			nameI := name + "-" + strconv.Itoa(i)
			service := types.NewManifestService(nameI, images[i])
			if crontab == "" {
				kubes = append(kubes, kube.NewJob(c.ns, service))
			} else {
				kubes = append(kubes, kube.NewCronJob(c.ns, service, crontab))
			}
		}
	}

	return c.run(kubes)
}

func (c *Client) UpdateTask(name, image string, replicas, internalPort, externalPort uint32) error {
	kubes := []kube.Kube{kube.NewPrepare(c.ns, &types.ManifestService{Name: name})}

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

	kubes = append(kubes, kube.NewDeployment(c.ns, service))
	kubes = append(kubes, kube.NewService(c.ns, service, service.Expose[0]))
	kubes = append(kubes, kube.NewIngress(c.ns, service, service.Expose[0]))
	return c.run(kubes)
}

func (c *Client) CancelTask(name string) error {
	service := types.NewManifestService(name, "")
	service.Count = 0
	expose := &types.ManifestServiceExpose{}

	if err := kube.NewDeployment(c.ns, service).Delete(c.kc); err != nil {
		return err
	}
	if err := kube.NewService(c.ns, service, expose).Delete(c.kc); err != nil {
		return err
	}
	if err := kube.NewIngress(c.ns, service, expose).Delete(c.kc); err != nil {
		return err
	}
	return nil
}

func (c *Client) ListTask() ([]string, error) {
	res := &appsv1.DeploymentList{}
	if err := kube.NewDeployment(c.ns, &types.ManifestService{}).List(c.kc, res); err != nil {
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

func (c *Client) Metering() (map[string]*types.ResourceUnit, error) {
	result := &map[string]*types.ResourceUnit{}
	if err := kube.NewMetering(c.ns, &types.ManifestService{}).List(c.kc, result); err != nil {
		return nil, err
	}
	return *result, nil
}
