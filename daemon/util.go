package daemon

//go:generate stringer -type taskType
type taskType int

const (
	NewTask taskType = iota
	UpdateTask
	CancelTask
	ListTask
	HeartBeat
)

//go:generate stringer -type msgStatus
type msgStatus int

const (
	StartFailure msgStatus = iota
	StartSuccess
	CancelFailure
	Cancelled
	UpdateFailure
	UpdateSuccess
)

func TendermintKey(dcName, namespace string) string {
	return dcName + ":" + namespace
}
