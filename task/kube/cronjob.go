package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cronJob struct {
	*common
	service  *types.ManifestService
	schedule string

	*batchv1beta1.CronJob
}

func NewCronJob(namespace string, service *types.ManifestService, schedule string) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &cronJob{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service:  service,
		schedule: schedule,
	}
}

func (k *cronJob) build() {
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

func (k *cronJob) Create(c *Client) error {
	k.build()
	_, err := c.BatchV1beta1().CronJobs(k.ns()).Create(k.CronJob)
	return errors.Wrap(err, "create job")
}

func (k *cronJob) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update job") }()

	obj, err := c.BatchV1beta1().CronJobs(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.CronJob = obj.DeepCopy()
	k.CronJob.Labels = k.labels()

	_, err = c.BatchV1beta1().CronJobs(k.ns()).Update(k.CronJob)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err := c.BatchV1beta1().CronJobs(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *cronJob) Delete(c *Client) error {
	err := c.BatchV1beta1().CronJobs(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete job")
}
func (k *cronJob) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	err := c.BatchV1beta1().CronJobs(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete job collection")
}

func (k *cronJob) List(c *Client, result interface{}) error {
	list, err := c.BatchV1beta1().CronJobs(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list cronJob")
	}

	*(result.(*batchv1beta1.CronJobList)) = *list
	return nil
}
