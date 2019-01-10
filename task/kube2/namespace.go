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

	k := &Namespace{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
	k.build()
	return k
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

func (k *Namespace) Update(kc kubernetes.Interface) (err error) {
	defer errors.Wrap(err, "update namespace")

	obj, err := kc.CoreV1().Namespaces().Get(k.name(), metav1.GetOptions{})
	if k.needCreate(err) {
		k.build()
		return k.Create(kc)
	}
	if err != nil {
		return err
	}

	obj.Name = k.ns()
	obj.Labels = k.labels()

	k.Namespace = obj
	_, err = kc.CoreV1().Namespaces().Update(k.Namespace)
	return err
}

func (k *Namespace) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	panic("delete namespace collection is dangerous")
}

// Delete is a api not in Kube definition
func (k *Namespace) Delete(kc kubernetes.Interface, namespace string) error {
	err := kc.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	return errors.Wrapf(err, "delete namespace(%s)", namespace)
}

func (k *Namespace) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list namespace")
	}

	*(result.(*corev1.NamespaceList)) = *list
	return nil
}
