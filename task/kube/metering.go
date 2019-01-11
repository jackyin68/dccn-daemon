package kube

import (
	"time"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Metering struct {
	*common
}

func NewMetering(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &Metering{
		common: &common{
			namespace: namespace,
			service:   service,
		},
	}
}

func (k *Metering) Create(kc kubernetes.Interface) error {
	return nil
}

func (k *Metering) Update(kc kubernetes.Interface) (rollback func(kc kubernetes.Interface) error, err error) {
	return nil, nil
}

func (k *Metering) Delete(kc kubernetes.Interface) error {
	return nil
}
func (k *Metering) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	return nil
}

func (k *Metering) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.CoreV1().Pods(k.ns()).List(metav1.ListOptions{
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
