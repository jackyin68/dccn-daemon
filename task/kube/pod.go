package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type pod struct {
	*common
	expose *types.ManifestServiceExpose

	*corev1.Pod
}

// FIXME: definition of pod
func NewPod(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &pod{
		common: &common{
			namespace: namespace,
			service:   service,
		},
	}
}

func (k *pod) Create(c *Client) error {
	_, err := c.CoreV1().Pods(k.ns()).Create(k.Pod)
	return errors.Wrap(err, "create pod")
}

func (k *pod) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update pod") }()

	obj, err := c.CoreV1().Pods(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Pod = obj.DeepCopy()
	k.Pod.Labels = k.labels()

	_, err = c.CoreV1().Pods(k.ns()).Update(k.Pod)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err = c.CoreV1().Pods(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *pod) Delete(c *Client) error {
	err := c.CoreV1().Pods(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete pod")
}
func (k *pod) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	err := c.CoreV1().Pods(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete pod collection")
}

func (k *pod) List(c *Client, result interface{}) error {
	list, err := c.CoreV1().Pods(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list pod")
	}

	*(result.(*corev1.PodList)) = *list
	return nil
}
