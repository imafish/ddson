package task

type SubTask interface {
	GetID() int

	Prepare() error
	Run() error
}
