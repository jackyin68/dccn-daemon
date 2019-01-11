package kube

import (
	"strconv"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type Service struct {
	*common
	expose *types.ManifestServiceExpose

	*corev1.Service
}

func NewService(namespace string, service *types.ManifestService, expose *types.ManifestServiceExpose) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &Service{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		expose: expose,
	}
}

func (k *Service) build() {
	k.Service = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.name(),
			Labels: k.labels(),
		},
		Spec: corev1.ServiceSpec{
			Selector: k.labels(),
			Ports:    k.ports(),
		},
	}
}
func (k *Service) ports() []corev1.ServicePort {
	ports := make([]corev1.ServicePort, 0, len(k.service.Expose))
	for _, expose := range k.service.Expose {
		ports = append(ports, corev1.ServicePort{
			Name:       strconv.Itoa(int(expose.Port)),
			Port:       exposeExternalPort(k.expose),
			TargetPort: intstr.FromInt(int(expose.Port)),
		})
	}
	return ports
}

func (k *Service) Create(kc kubernetes.Interface) error {
	k.build()
	_, err := kc.CoreV1().Services(k.ns()).Create(k.Service)
	return errors.Wrap(err, "create service")
}

func (k *Service) Update(kc kubernetes.Interface) (rollback func(kc kubernetes.Interface) error, err error) {
	defer func() { err = errors.Wrap(err, "update service") }()

	obj, err := kc.CoreV1().Services(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Service = obj.DeepCopy()
	k.Service.Labels = k.labels()
	k.Service.Spec.Selector = k.labels()
	k.Service.Spec.Ports = k.ports()

	_, err = kc.CoreV1().Services(k.ns()).Update(k.Service)
	if err != nil {
		return nil, err
	}

	return func(kc kubernetes.Interface) error {
		_, err = kc.CoreV1().Services(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *Service) Delete(kc kubernetes.Interface) error {
	err := kc.CoreV1().Services(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete service")
}
func (k *Service) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) (err error) {
	defer func() { err = errors.Wrap(err, "delete service collection") }()

	services, err := kc.CoreV1().Services(k.ns()).List(selector)
	if err != nil {
		return err
	}

	for _, item := range services.Items {
		err := kc.CoreV1().Services(k.ns()).Delete(item.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *Service) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.CoreV1().Services(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list service")
	}

	*(result.(*corev1.ServiceList)) = *list
	return nil
}
