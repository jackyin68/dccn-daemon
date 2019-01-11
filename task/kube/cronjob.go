package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CronJob struct {
	*common
	service  *types.ManifestService
	schedule string

	*batchv1beta1.CronJob
}

func NewCronJob(namespace string, service *types.ManifestService, schedule string) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &CronJob{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service:  service,
		schedule: schedule,
	}
}

func (k *CronJob) build() {
	k.CronJob = &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:   k.name(),
			Labels: k.labels(),
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule: k.schedule,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
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
			},
		},
	}
}

func (k *CronJob) Create(kc kubernetes.Interface) error {
	k.build()
	_, err := kc.BatchV1beta1().CronJobs(k.ns()).Create(k.CronJob)
	return errors.Wrap(err, "create job")
}

func (k *CronJob) Update(kc kubernetes.Interface) (rollback func(kc kubernetes.Interface) error, err error) {
	defer func() { err = errors.Wrap(err, "update job") }()

	obj, err := kc.BatchV1beta1().CronJobs(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.CronJob = obj.DeepCopy()
	k.CronJob.Labels = k.labels()

	_, err = kc.BatchV1beta1().CronJobs(k.ns()).Update(k.CronJob)
	if err != nil {
		return nil, err
	}

	return func(kc kubernetes.Interface) error {
		_, err := kc.BatchV1beta1().CronJobs(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *CronJob) Delete(kc kubernetes.Interface) error {
	err := kc.BatchV1beta1().CronJobs(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete job")
}
func (k *CronJob) DeleteCollection(kc kubernetes.Interface, selector metav1.ListOptions) error {
	err := kc.BatchV1beta1().CronJobs(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete job collection")
}

func (k *CronJob) List(kc kubernetes.Interface, result interface{}) error {
	list, err := kc.BatchV1beta1().CronJobs(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list cronjob")
	}

	*(result.(*batchv1beta1.CronJobList)) = *list
	return nil
}
