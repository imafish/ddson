package downloadtask

type TaskManager interface {
	AddTask(task *Task) int
	GetTaskByID(taskID int) (Task, error)

	GetPendingTasks() []Task
	GetRunningTasks() []Task
	GetCompletedTasks() []Task

	// a loop that executes tasks
	Run(func(task *Task) error)
}
