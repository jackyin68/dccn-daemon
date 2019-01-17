package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type namespace struct {
	*common
	service *types.ManifestService

	*corev1.Namespace
}

func NewNamespace(ns string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &namespace{
		common: &common{
			namespace: ns,
			service:   service,
		},
		service: service,
	}
}

func (k *namespace) build() {
	k.Namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: k.ns(),
		},
	}
}

func (k *namespace) Create(c *Client) error {
	k.build()
	_, err := c.CoreV1().Namespaces().Create(k.Namespace)
	return errors.Wrap(err, "create namespace")
}

func (k *namespace) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update namespace") }()

	obj, err := c.CoreV1().Namespaces().Get(k.ns(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Namespace = obj.DeepCopy()
	k.Namespace.Name = k.ns()
	k.Namespace.Labels = k.labels()

	_, err = c.CoreV1().Namespaces().Update(k.Namespace)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err = c.CoreV1().Namespaces().Update(obj)
		return err
	}, nil
}

func (k *namespace) Delete(c *Client) error {
	err := c.CoreV1().Namespaces().Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrapf(err, "delete namespace(%s)", k.name())
}
func (k *namespace) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	return errors.New("delete namespace collection is dangerous")
}

func (k *namespace) List(c *Client, result interface{}) error {
	list, err := c.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list namespace")
	}

	*(result.(*corev1.NamespaceList)) = *list
	return nil
}
