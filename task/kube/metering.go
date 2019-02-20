package kube

import (
	"time"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type metering struct {
	*common
}

func NewMetering(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &metering{
		common: &common{
			namespace: namespace,
			service:   service,
		},
	}
}

func (k *metering) Create(c *Client) error {
	return nil
}

func (k *metering) Update(c *Client) (rollback func(c *Client) error, err error) {
	return nil, nil
}

func (k *metering) Delete(c *Client) error {
	return nil
}
func (k *metering) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	return nil
}

func (k *metering) List(c *Client, result interface{}) error {
	list, err := c.CoreV1().Pods(k.ns()).List(metav1.ListOptions{
		LabelSelector: Selector(),
	})
	if err != nil {
		return errors.Wrap(err, "list pods")
	}

	now := time.Now()
	res := &map[string]*types.ResourceUnit{}
	for _, item := range list.Items {
		var cpu, mem, disk int64
		for _, status := range item.Status.ContainerStatuses {
			if status.State.Running == nil {
				continue
			}

			nano := int64(now.Sub(status.State.Running.StartedAt.Time).Seconds())

			for _, container := range item.Spec.Containers {
				if container.Name != status.Name {
					continue
				}

				limits := container.Resources.Limits
				cpu += nano * limits.Cpu().MilliValue() // unit: types.Core
				mem += nano * limits.Memory().MilliValue() / 1000
				disk += nano * limits.StorageEphemeral().MilliValue() / 1000
			}
			(*res)[status.Name] = &types.ResourceUnit{
				CPU:    uint32(cpu),
				Memory: uint64(mem),
				Disk:   uint64(disk),
			}
		}
	}

	*(result.(*map[string]*types.ResourceUnit)) = *res
	return nil
}
