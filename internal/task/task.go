package task

type Task interface {
	GetID() int

	Prepare() error
	Run() error
}
