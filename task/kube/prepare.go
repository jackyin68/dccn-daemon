package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

type Prepare struct {
	*common
	service *types.ManifestService
}

func NewPrepare(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &Prepare{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
}

func (k *Prepare) Create(kc kubernetes.Interface) error {
	return nil
}

// FIXME: logic
func (k *Prepare) Update(kc kubernetes.Interface) (rollback func(kc kubernetes.Interface) error, err error) {
	defer func() { err = errors.Wrap(err, "prepare env") }()

	// prepare namespace
	_, err = NewNamespace(k.ns(), k.service).Update(kc)
	if k8sErr.IsNotFound(err) {
		err = NewNamespace(k.ns(), k.service).Create(kc)
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
	if err = (&Deployment{}).DeleteCollection(kc, selector); err != nil {
		return
	}
	if err = (&Ingress{}).DeleteCollection(kc, selector); err != nil {
		return
	}
	if err = (&Service{}).DeleteCollection(kc, selector); err != nil {
		return
	}
	if err = (&Job{}).DeleteCollection(kc, selector); err != nil {
		return
	}
	if err = (&CronJob{}).DeleteCollection(kc, selector); err != nil {
		return
	}

	return
}

func (k *Prepare) Delete(kc kubernetes.Interface) error {
	return nil
}
func (k *Prepare) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	return nil
}

func (k *Prepare) List(kc kubernetes.Interface, result interface{}) error {
	return nil
}
