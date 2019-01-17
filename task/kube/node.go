package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type node struct {
	*common
	service *types.ManifestService
}

func NewNode(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &node{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
}

func (k *node) Create(c *Client) error {
	return nil
}

func (k *node) Update(c *Client) (rollback func(c *Client) error, err error) {
	return nil, nil
}

func (k *node) Delete(c *Client) error {
	return nil
}
func (k *node) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	return nil
}

func (k *node) List(c *Client, result interface{}) error {
	list, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list node")
	}

	*(result.(*corev1.NodeList)) = *list
	return nil
}
