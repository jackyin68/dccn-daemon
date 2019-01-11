package task

import (
	"reflect"
	"testing"

	"github.com/Ankr-network/dccn-daemon/task/kube"
	"github.com/Ankr-network/dccn-daemon/types"
	"github.com/golang/mock/gomock"
)

func TestRunner_CreateTasks(t *testing.T) {
	type fields struct {
		client kube.Client
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := kube.NewMockClient(ctrl)
	mockFields := fields{client}

	type args struct {
		name   string
		images []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			fields:  mockFields,
			args:    args{"test", []string{"nginx:1.12"}},
			wantErr: false,
		},
	}
	client.EXPECT().Deploy(gomock.Any()).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				client: tt.fields.client,
			}
			if err := r.CreateTasks(tt.args.name, tt.args.images...); (err != nil) != tt.wantErr {
				t.Errorf("Runner.CreateTasks() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunner_UpdateTask(t *testing.T) {
	type fields struct {
		client kube.Client
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := kube.NewMockClient(ctrl)
	mockFields := fields{client}

	type args struct {
		name         string
		image        string
		replicas     uint32
		internalPort uint32
		externalPort uint32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			fields:  mockFields,
			args:    args{"test", "nginx:1.12", 3, 0, 0},
			wantErr: false,
		},
	}
	client.EXPECT().Deploy(gomock.Any()).Return(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				client: tt.fields.client,
			}
			if err := r.UpdateTask(tt.args.name, tt.args.image, tt.args.replicas, tt.args.internalPort, tt.args.externalPort); (err != nil) != tt.wantErr {
				t.Errorf("Runner.UpdateTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunner_ListTask(t *testing.T) {
	type fields struct {
		client kube.Client
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := kube.NewMockClient(ctrl)
	mockFields := fields{client}

	tests := []struct {
		name    string
		fields  fields
		want    []string
		wantErr bool
	}{
		{
			fields:  mockFields,
			want:    []string{"content"},
			wantErr: false,
		},
	}
	client.EXPECT().ListDeployments().Return([]string{"test"}, []string{"content"}, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				client: tt.fields.client,
			}
			got, err := r.ListTask()
			if (err != nil) != tt.wantErr {
				t.Errorf("Runner.ListTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Runner.ListTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunner_Metering(t *testing.T) {
	type fields struct {
		client kube.Client
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := kube.NewMockClient(ctrl)
	mockFields := fields{client}

	tests := []struct {
		name    string
		fields  fields
		want    map[string]*types.ResourceUnit
		wantErr bool
	}{
		{
			fields: mockFields,
			want: map[string]*types.ResourceUnit{
				"test": nil,
			},
			wantErr: false,
		},
	}
	client.EXPECT().Metering().Return(map[string]*types.ResourceUnit{
		"test": nil,
	}, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				client: tt.fields.client,
			}
			got, err := r.Metering()
			if (err != nil) != tt.wantErr {
				t.Errorf("Runner.Metering() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Runner.Metering() = %v, want %v", got, tt.want)
			}
		})
	}
}
