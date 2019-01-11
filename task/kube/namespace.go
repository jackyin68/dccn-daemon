package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Namespace struct {
	*common
	service *types.ManifestService

	*corev1.Namespace
}

func NewNamespace(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &Namespace{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
}

func (k *Namespace) build() {
	k.Namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.ns(),
			Labels: k.labels(),
		},
	}
}

func (k *Namespace) Create(kc kubernetes.Interface) error {
	_, err := kc.CoreV1().Namespaces().Create(k.Namespace)
	return errors.Wrap(err, "create namespace")
}

func (k *Namespace) Update(kc kubernetes.Interface) (rollback func(kc kubernetes.Interface) error, err error) {
	defer func() { err = errors.Wrap(err, "update namespace") }()

	obj, err := kc.CoreV1().Namespaces().Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Namespace = obj.DeepCopy()
	k.Namespace.Name = k.ns()
	k.Namespace.Labels = k.labels()

	_, err = kc.CoreV1().Namespaces().Update(k.Namespace)
	if err != nil {
		return nil, err
	}

	return func(kc kubernetes.Interface) error {
		_, err = kc.CoreV1().Namespaces().Update(obj)
		return err
	}, nil
}

func (k *Namespace) Delete(kc kubernetes.Interface) error {
	err := kc.CoreV1().Namespaces().Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrapf(err, "delete namespace(%s)", k.name())
}
func (k *Namespace) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	return errors.New("delete namespace collection is dangerous")
}

func (k *Namespace) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list namespace")
	}

	*(result.(*corev1.NamespaceList)) = *list
	return nil
}
