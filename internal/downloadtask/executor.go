package downloadtask

type SubtaskExecutor interface {
	Run(task *Task, subtask *subTask)
}

type defaultSubtaskExecutor struct{}

func (defaultSubtaskExecutor) Run(task *Task, subtask *subTask) {
	runSubtask(task, subtask)
}
