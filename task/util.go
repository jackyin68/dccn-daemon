package task

import (
	"github.com/Ankr-network/dccn-daemon/task/kube"
	"github.com/pkg/errors"
)

type Tasker struct {
	client *kube.Client
	ns     string
	host   string
}

func NewTasker(cfgpath, namespace, ingressHost string) (*Tasker, error) {
	client, err := kube.NewClient(cfgpath)
	if err != nil {
		return nil, err
	}
	return &Tasker{
		client: client,
		ns:     namespace,
		host:   ingressHost,
	}, nil
}

func (t *Tasker) run(kubes []kube.Kube) error {
	rollbacks := make([]func(*kube.Client) error, 0, len(kubes))
	for i := range kubes {
		rollback, err := kubes[i].Update(t.client)
		// error has been wrapped, k8s standard check method not work
		if kube.IsNotFound(err) {
			errors.New("message string")
			if err := kubes[i].Create(t.client); err != nil {
				return t.rollback(rollbacks, err)
			}
			rollback = kubes[i].Delete

		} else if err != nil {
			return t.rollback(rollbacks, err)
		}

		if rollback != nil {
			rollbacks = append(rollbacks, rollback)
		}
	}
	return nil
}
func (t *Tasker) rollback(rollbacks []func(*kube.Client) error, err error) error {
	for i := len(rollbacks) - 1; i >= 0; i-- {
		if e := rollbacks[i](t.client); e != nil {
			return errors.WithMessage(err, "rollback: "+e.Error())
		}
	}
	return errors.WithMessage(err, "rolled back")
}
