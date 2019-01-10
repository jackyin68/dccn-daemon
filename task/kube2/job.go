package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Job struct {
	*common
	service *types.ManifestService

	*batchv1.Job
}

func NewJob(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	k := &Job{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
	k.build()
	return k
}

func (k *Job) build() {
	k.Job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.name(),
			Labels: k.labels(),
		},
		Spec: batchv1.JobSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: k.labels(),
			},
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

func (k *Job) Create(kc kubernetes.Interface) error {
	_, err := kc.BatchV1().Jobs(k.ns()).Create(k.Job)
	return errors.Wrap(err, "create job")
}

func (k *Job) Update(kc kubernetes.Interface) (err error) {
	defer errors.Wrap(err, "update job")

	obj, err := kc.BatchV1().Jobs(k.ns()).Get(k.name(), metav1.GetOptions{})
	if k.needCreate(err) {
		k.build()
		return k.Create(kc)
	}
	if err != nil {
		return err
	}

	obj.Labels = k.labels()

	k.Job = obj
	_, err = kc.BatchV1().Jobs(k.ns()).Update(k.Job)
	return err
}

func (k *Job) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	err := kc.BatchV1().Jobs(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete collection job")
}

func (k *Job) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.BatchV1().Jobs(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list job")
	}

	*(result.(*batchv1.JobList)) = *list
	return nil
}
