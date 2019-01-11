package kube

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func applyNS(kc kubernetes.Interface, b *nsBuilder) error {
	obj, err := kc.CoreV1().Namespaces().Get(b.name(), metav1.GetOptions{})
	switch {
	case err == nil:
		obj, err = b.update(obj)
		if err == nil {
			_, err = kc.CoreV1().Namespaces().Update(obj)
			err = errors.Wrap(err, "update")
		}
	case k8sErr.IsNotFound(err):
		obj, err = b.create()
		if err == nil {
			_, err = kc.CoreV1().Namespaces().Create(obj)
			err = errors.Wrap(err, "create")
		}
	default:
		err = errors.Wrap(err, "get")
	}
	return err
}

func applyDeployment(kc kubernetes.Interface, b *deploymentBuilder) error {
	obj, err := kc.AppsV1().Deployments(b.ns()).Get(b.name(), metav1.GetOptions{})
	switch {
	case err == nil && b.service.Count == 0:
		err = kc.AppsV1().Deployments(b.ns()).Delete(b.service.Name, &metav1.DeleteOptions{})
		err = errors.Wrap(err, "delete")
	case err == nil:
		obj, err = b.update(obj)
		if err == nil {
			_, err = kc.AppsV1().Deployments(b.ns()).Update(obj)
			err = errors.Wrap(err, "update")
		}
	case k8sErr.IsNotFound(err):
		obj, err = b.create()
		if err == nil {
			_, err = kc.AppsV1().Deployments(b.ns()).Create(obj)
			err = errors.Wrap(err, "create")
		}
	default:
		err = errors.Wrap(err, "get")
	}
	return err
}

func applyService(kc kubernetes.Interface, b *serviceBuilder) error {
	obj, err := kc.CoreV1().Services(b.ns()).Get(b.name(), metav1.GetOptions{})
	switch {
	case err == nil && len(b.service.Expose) == 0:
		err = kc.CoreV1().Services(b.ns()).Delete(b.service.Name, &metav1.DeleteOptions{})
		err = errors.Wrap(err, "delete")
	case err == nil:
		obj, err = b.update(obj)
		if err == nil {
			_, err = kc.CoreV1().Services(b.ns()).Update(obj)
			err = errors.Wrap(err, "update")
		}
	case k8sErr.IsNotFound(err):
		obj, err = b.create()
		if err == nil {
			_, err = kc.CoreV1().Services(b.ns()).Create(obj)
			err = errors.Wrap(err, "create")
		}
	default:
		err = errors.Wrap(err, "get")
	}
	return err
}

func applyIngress(kc kubernetes.Interface, b *ingressBuilder) error {
	obj, err := kc.ExtensionsV1beta1().Ingresses(b.ns()).Get(b.name(), metav1.GetOptions{})
	switch {
	case err == nil && len(b.service.Expose) == 0:
		err = kc.ExtensionsV1beta1().Ingresses(b.ns()).Delete(b.service.Name, &metav1.DeleteOptions{})
		err = errors.Wrap(err, "delete")
	case err == nil:
		obj, err = b.update(obj)
		if err == nil {
			_, err = kc.ExtensionsV1beta1().Ingresses(b.ns()).Update(obj)
			err = errors.Wrap(err, "update")
		}
	case k8sErr.IsNotFound(err):
		obj, err = b.create()
		if err == nil {
			_, err = kc.ExtensionsV1beta1().Ingresses(b.ns()).Create(obj)
			err = errors.Wrap(err, "create")
		}
	default:
		err = errors.Wrap(err, "get")
	}
	return err
}

func prepareEnvironment(kc kubernetes.Interface, ns string) error {
	_, err := kc.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if k8sErr.IsNotFound(err) {
		obj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		_, err = kc.CoreV1().Namespaces().Create(obj)
		return errors.Wrap(err, "create")
	}
	return errors.Wrap(err, "get")
}
