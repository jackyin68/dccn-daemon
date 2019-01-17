package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type prepare struct {
	*common
	service *types.ManifestService
}

func NewPrepare(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &prepare{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
}

func (k *prepare) Create(c *Client) error {
	return nil
}

// FIXME: logic
func (k *prepare) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "prepare env") }()

	// prepare namespace
	_, err = NewNamespace(k.ns(), k.service).Update(c)
	if k8sErr.IsNotFound(err) {
		err = NewNamespace(k.ns(), k.service).Create(c)
		rollback = NewNamespace(k.ns(), k.service).Delete
	}
	if err != nil {
		return
	}

	// cleanupStaleResources
	// build label selector for objects not in current manifest group
	svcnames := []string{k.service.Name}
	req1, err := labels.NewRequirement(manifestServiceLabelName, selection.NotIn, svcnames)
	if err != nil {
		return
	}
	req2, err := labels.NewRequirement(managedLabelName, selection.Equals, []string{"true"})
	if err != nil {
		return
	}
	selector := metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req1).Add(*req2).String(),
	}

	// delete stale resource
	// FIXME: init structure
	if err = (&deployment{}).DeleteCollection(c, selector); err != nil {
		return
	}
	if err = (&ingress{}).DeleteCollection(c, selector); err != nil {
		return
	}
	if err = (&service{}).DeleteCollection(c, selector); err != nil {
		return
	}
	if err = (&job{}).DeleteCollection(c, selector); err != nil {
		return
	}
	if err = (&cronJob{}).DeleteCollection(c, selector); err != nil {
		return
	}

	return
}

func (k *prepare) Delete(c *Client) error {
	return nil
}
func (k *prepare) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	return nil
}

func (k *prepare) List(c *Client, result interface{}) error {
	return nil
}
