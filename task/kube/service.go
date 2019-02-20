package kube

import (
	"strconv"

	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type service struct {
	*common
	expose *types.ManifestServiceExpose

	*corev1.Service
}

func NewService(namespace string, svc *types.ManifestService, expose *types.ManifestServiceExpose) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &service{
		common: &common{
			namespace: namespace,
			service:   svc,
		},
		expose: expose,
	}
}

func (k *service) build() {
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
func (k *service) ports() []corev1.ServicePort {
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

func (k *service) Create(c *Client) error {
	k.build()
	_, err := c.CoreV1().Services(k.ns()).Create(k.Service)
	return errors.Wrap(err, "create service")
}

func (k *service) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update service") }()

	obj, err := c.CoreV1().Services(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Service = obj.DeepCopy()
	k.Service.Labels = k.labels()
	k.Service.Spec.Selector = k.labels()
	k.Service.Spec.Ports = k.ports()

	_, err = c.CoreV1().Services(k.ns()).Update(k.Service)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err = c.CoreV1().Services(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *service) Delete(c *Client) error {
	err := c.CoreV1().Services(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete service")
}
func (k *service) DeleteCollection(c *Client, selector metav1.ListOptions) (err error) {
	defer func() { err = errors.Wrap(err, "delete service collection") }()

	services, err := c.CoreV1().Services(k.ns()).List(selector)
	if err != nil {
		return err
	}

	for _, item := range services.Items {
		err := c.CoreV1().Services(k.ns()).Delete(item.Name, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *service) List(c *Client, result interface{}) error {
	list, err := c.CoreV1().Services(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list service")
	}

	*(result.(*corev1.ServiceList)) = *list
	return nil
}
