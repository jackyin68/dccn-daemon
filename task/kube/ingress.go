package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	extv1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type ingress struct {
	*common
	expose *types.ManifestServiceExpose

	*extv1.Ingress
}

func NewIngress(namespace string, service *types.ManifestService, expose *types.ManifestServiceExpose) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &ingress{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		expose: expose,
	}
}

func (k *ingress) build() {
	k.Ingress = &extv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.name(),
			Labels: k.labels(),
		},
		Spec: extv1.IngressSpec{
			Backend: &extv1.IngressBackend{
				ServiceName: k.name(),
				ServicePort: intstr.FromInt(int(exposeExternalPort(k.expose))),
			},
			Rules: k.rules(),
		},
	}
}
func (k *ingress) rules() []extv1.IngressRule {
	rules := make([]extv1.IngressRule, 0, len(k.expose.Hosts))
	for _, host := range k.expose.Hosts {
		rules = append(rules, extv1.IngressRule{Host: host})
	}
	return rules
}

func (k *ingress) Create(c *Client) error {
	k.build()
	_, err := c.ExtensionsV1beta1().Ingresses(k.ns()).Create(k.Ingress)
	return errors.Wrap(err, "create ingress")
}

func (k *ingress) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update ingress") }()

	obj, err := c.ExtensionsV1beta1().Ingresses(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Ingress = obj.DeepCopy()
	k.Ingress.Labels = k.labels()
	k.Ingress.Spec.Backend.ServicePort = intstr.FromInt(int(exposeExternalPort(k.expose)))
	k.Ingress.Spec.Rules = k.rules()

	_, err = c.ExtensionsV1beta1().Ingresses(k.ns()).Update(k.Ingress)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err = c.ExtensionsV1beta1().Ingresses(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *ingress) Delete(c *Client) error {
	err := c.ExtensionsV1beta1().Ingresses(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete ingress")
}
func (k *ingress) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	err := c.ExtensionsV1beta1().Ingresses(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete ingress collection")
}

func (k *ingress) List(c *Client, result interface{}) error {
	list, err := c.ExtensionsV1beta1().Ingresses(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list ingress")
	}

	*(result.(*extv1.IngressList)) = *list
	return nil
}
