package task

import (
	"sync"
	"testing"
)

func TestTasker_AddPrefix(t *testing.T) {
	type fields struct {
		dcName string
	}
	type args struct {
		name string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{{
		"Upper",
		fields{"Upper"},
		args{"123"},
		"U123",
	}, {
		"lower",
		fields{"1lower"},
		args{"123"},
		"L123",
	}, {
		"Ankr",
		fields{""},
		args{"123"},
		"A123",
	}, {
		"Ankr",
		fields{"1234"},
		args{"123"},
		"A123",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasker := &Tasker{
				dcName: tt.fields.dcName,
			}
			once = sync.Once{}
			if got := tasker.AddPrefix(tt.args.name); got != tt.want {
				t.Errorf("Tasker.AddPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}
