package task

type TaskList interface {
	AddTask(task Task) error
	GetOneTask() Task
}
