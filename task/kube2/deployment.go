package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Deployment struct {
	*common
	service *types.ManifestService

	*appsv1.Deployment
}

func NewDeployment(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	k := &Deployment{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
	k.build()
	return k
}

func (k *Deployment) build() {
	replicas := int32(k.service.Count)
	k.Deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.name(),
			Labels: k.labels(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: k.labels(),
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: k.labels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{k.container()},
				},
			},
		},
	}
}

func (k *Deployment) Create(kc kubernetes.Interface) error {
	_, err := kc.AppsV1().Deployments(k.ns()).Create(k.Deployment)
	return errors.Wrap(err, "create deployment")
}

func (k *Deployment) Update(kc kubernetes.Interface) (err error) {
	defer errors.Wrap(err, "update deployment")

	obj, err := kc.AppsV1().Deployments(k.ns()).Get(k.name(), metav1.GetOptions{})
	if k.needCreate(err) {
		k.build()
		return k.Create(kc)
	}
	if err != nil {
		return err
	}

	replicas := int32(k.service.Count)
	obj.Labels = k.labels()
	obj.Spec.Selector.MatchLabels = k.labels()
	obj.Spec.Replicas = &replicas
	obj.Spec.Template.Labels = k.labels()
	obj.Spec.Template.Spec.Containers = []corev1.Container{k.container()}

	k.Deployment = obj
	_, err = kc.AppsV1().Deployments(k.ns()).Update(k.Deployment)
	return err
}

func (k *Deployment) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	err := kc.AppsV1().Deployments(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete deployment collection")
}

func (k *Deployment) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.AppsV1().Deployments(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list deployment")
	}

	*(result.(*appsv1.DeploymentList)) = *list
	return nil
}
