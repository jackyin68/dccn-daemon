package kube

import (
	"time"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type metrics struct {
	*common
}

func NewMetrics(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &metrics{
		common: &common{
			namespace: namespace,
			service:   service,
		},
	}
}

func (k *metrics) Create(c *Client) error {
	return nil
}

func (k *metrics) Update(c *Client) (rollback func(c *Client) error, err error) {
	return nil, nil
}

func (k *metrics) Delete(c *Client) error {
	return nil
}
func (k *metrics) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	return nil
}

type Metric struct {
	CPU              int64
	Memory           int64
	Storage          int64
	EphemeralStorage int64
}

type Metrics struct {
	NodeImages map[string][][]string
	NodeTotal  map[string]*Metric
	NodeInUse  map[string]*Metric
	Endpoints  map[int32]int64
}

func (k *metrics) List(c *Client, result interface{}) (err error) {
	defer func() { err = errors.Wrap(err, "list metrics") }()

	res := &Metrics{
		NodeImages: map[string][][]string{},
		NodeTotal:  map[string]*Metric{},
		NodeInUse:  map[string]*Metric{},
		Endpoints:  map[int32]int64{},
	}
	{
		nodeList := &corev1.NodeList{}
		if err := NewNode(k.ns(), k.service).List(c, nodeList); err != nil {
			return err
		}
		for _, item := range nodeList.Items {
			images := make([][]string, 0, len(nodeList.Items))
			for _, image := range item.Status.Images {
				images = append(images, image.Names)
			}
			res.NodeImages[item.Name] = images

			metric := &Metric{}
			for resource, quantity := range item.Status.Allocatable {
				switch corev1.ResourceName(resource) {
				case corev1.ResourceCPU:
					metric.CPU = quantity.MilliValue()
				case corev1.ResourceMemory:
					metric.Memory = quantity.MilliValue() / 1000
				case corev1.ResourceStorage:
					metric.Storage = quantity.MilliValue() / 1000
				case corev1.ResourceEphemeralStorage:
					metric.EphemeralStorage = quantity.MilliValue() / 1000
				}
			}
			res.NodeTotal[item.Name] = metric
		}
	}

	{
		services, err := c.CoreV1().Services(k.ns()).List(metav1.ListOptions{
			LabelSelector: Selector(),
		})
		if err != nil {
			return errors.Wrap(err, "list pods")
		}
		for _, item := range services.Items {
			for _, port := range item.Spec.Ports {
				res.Endpoints[port.Port] = res.Endpoints[port.Port] + 1
			}
		}
	}
	{
		metcs, err := c.metc.Metrics().NodeMetricses().List(metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "node metrics")
		}
		for _, item := range metcs.Items {
			metric := &Metric{}
			for resource, quantity := range item.Usage {
				switch corev1.ResourceName(resource) {
				case corev1.ResourceCPU:
					metric.CPU = quantity.MilliValue() / int64(item.Window.Duration/time.Second)
				case corev1.ResourceMemory:
					metric.Memory = quantity.MilliValue() / 1000
				case corev1.ResourceStorage:
					metric.Storage = quantity.MilliValue() / 1000
				case corev1.ResourceEphemeralStorage:
					metric.EphemeralStorage = quantity.MilliValue() / 1000
				}
			}
			res.NodeInUse[item.Name] = metric
		}
	}

	*(result.(*Metrics)) = *res
	return nil
}
