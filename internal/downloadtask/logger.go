package downloadtask

import "github.com/imafish/ddson/internal/logging"

var logger = logging.DefaultLogger()

func SetLogger(l logging.Logger) {
	if l != nil {
		logger = l
	}
}
