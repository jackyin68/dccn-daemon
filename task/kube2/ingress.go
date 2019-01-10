package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	extv1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type Ingress struct {
	*common
	expose *types.ManifestServiceExpose

	*extv1.Ingress
}

func NewIngress(namespace string, service *types.ManifestService, expose *types.ManifestServiceExpose) Kube {
	if mockKube != nil {
		return mockKube
	}

	k := &Ingress{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		expose: expose,
	}
	k.build()
	return k
}

func (k *Ingress) build() {
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
func (k *Ingress) rules() []extv1.IngressRule {
	rules := make([]extv1.IngressRule, 0, len(k.expose.Hosts))
	for _, host := range k.expose.Hosts {
		rules = append(rules, extv1.IngressRule{Host: host})
	}
	return rules
}

func (k *Ingress) Create(kc kubernetes.Interface) error {
	_, err := kc.ExtensionsV1beta1().Ingresses(k.ns()).Create(k.Ingress)
	return errors.Wrap(err, "create ingress")
}

func (k *Ingress) Update(kc kubernetes.Interface) (err error) {
	defer errors.Wrap(err, "update ingress")

	obj, err := kc.ExtensionsV1beta1().Ingresses(k.ns()).Get(k.name(), metav1.GetOptions{})
	if k.needCreate(err) {
		k.build()
		return k.Create(kc)
	}
	if err != nil {
		return err
	}

	obj.Labels = k.labels()
	obj.Spec.Backend.ServicePort = intstr.FromInt(int(exposeExternalPort(k.expose)))
	obj.Spec.Rules = k.rules()

	k.Ingress = obj
	_, err = kc.ExtensionsV1beta1().Ingresses(k.ns()).Update(k.Ingress)
	return err
}

func (k *Ingress) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	err := kc.ExtensionsV1beta1().Ingresses(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete ingress collection")
}

func (k *Ingress) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.ExtensionsV1beta1().Ingresses(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list ingress")
	}

	*(result.(*extv1.IngressList)) = *list
	return nil
}
