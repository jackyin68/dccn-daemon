package kube

import (
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type job struct {
	*common
	service *types.ManifestService

	*batchv1.Job
}

func NewJob(namespace string, service *types.ManifestService) Kube {
	if mockKube != nil {
		return mockKube
	}

	return &job{
		common: &common{
			namespace: namespace,
			service:   service,
		},
		service: service,
	}
}

func (k *job) build() {
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

func (k *job) Create(c *Client) error {
	k.build()
	_, err := c.BatchV1().Jobs(k.ns()).Create(k.Job)
	return errors.Wrap(err, "create job")
}

func (k *job) Update(c *Client) (rollback func(c *Client) error, err error) {
	defer func() { err = errors.Wrap(err, "update job") }()

	obj, err := c.BatchV1().Jobs(k.ns()).Get(k.name(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	k.Job = obj.DeepCopy()
	k.Job.Labels = k.labels()

	_, err = c.BatchV1().Jobs(k.ns()).Update(k.Job)
	if err != nil {
		return nil, err
	}

	return func(c *Client) error {
		_, err = c.BatchV1().Jobs(k.ns()).Update(obj)
		return err
	}, nil
}
func (k *job) Delete(c *Client) error {
	err := c.BatchV1().Jobs(k.ns()).Delete(k.name(), &metav1.DeleteOptions{})
	return errors.Wrap(err, "delete job")
}
func (k *job) DeleteCollection(c *Client, selector metav1.ListOptions) error {
	err := c.BatchV1().Jobs(k.ns()).DeleteCollection(&metav1.DeleteOptions{}, selector)
	return errors.Wrap(err, "delete collection job")
}

func (k *job) List(c *Client, result interface{}) error {
	list, err := c.BatchV1().Jobs(k.ns()).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "list job")
	}

	*(result.(*batchv1.JobList)) = *list
	return nil
}
