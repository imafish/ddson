package common

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"syscall"
)

func Daemonize(pidfile string, logfile string) error {
	if pidfile == "" {
		return fmt.Errorf("pidfile cannot be empty")
	}

	_, err := os.Stat(pidfile)
	if err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("pidfile %s already exists", pidfile)
	}

	// create the pidfile
	// TODO: should open file exclusively
	pidFile, err := os.Create(pidfile)
	if err != nil {
		return fmt.Errorf("failed to create pidfile %s: %v", pidfile, err)
	}
	defer pidFile.Close()

	var nullFileFd uintptr = 0
	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %v", err)
	}
	defer nullFile.Close()
	nullFileFd = nullFile.Fd()
	var (
		stdinFd  = nullFileFd
		stdoutFd = nullFileFd
		stderrFd = nullFileFd
	)

	// if logfile != "" {
	// 	var logFilePtr *os.File
	// 	logFilePtr, err = os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to open logfile %s: %v", logfile, err)
	// 	}
	// 	defer logFilePtr.Close()
	// 	stdoutFd = logFilePtr.Fd()
	// 	stderrFd = logFilePtr.Fd()
	// }

	// TODO: to use systemd styled daemonization, we should log to stdout and stderr
	// For now, we'll use lumberjack to directly log to a file.

	commandline := os.Args[0]
	args := make([]string, 0, len(os.Args))
	for _, arg := range os.Args {
		switch arg {
		case "-daemon", "--daemon":
		case "-d", "--d":
		case "--force", "-f":
		case "--pidfile":
		// case "--logfile":
		// case logfile:
		case pidfile:
			continue // Skip these arguments
		default:
			args = append(args, arg)
		}
	}

	args = append(args, "--logfile", logfile)

	slog.Debug("Daemonizing process",
		"commandline", commandline,
		"args", args)

	// Fork the process
	pid, err := syscall.ForkExec(commandline, args, &syscall.ProcAttr{
		Files: []uintptr{
			stdinFd,  // Redirect stdin
			stdoutFd, // Redirect stdout
			stderrFd, // Redirect stderr
		},
		Sys: &syscall.SysProcAttr{
			Setpgid: true, // Set the process group ID
		},
		Env: []string{
			fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
			fmt.Sprintf("USER=%s", os.Getenv("USER")),
			fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
			fmt.Sprintf("SUDO_USER=%s", os.Getenv("SUDO_USER")),
		}, // Pass the environment variables
	})
	if err != nil {
		return fmt.Errorf("failed to fork process: %v", err)
	}

	// Write the PID to the pidfile
	_, err = pidFile.WriteString(fmt.Sprintf("%d", pid))
	if err != nil {
		return fmt.Errorf("failed to write PID to pidfile %s: %v", pidfile, err)
	}

	slog.Info("Daemonized process started", "pid", pid, "pidfile", pidfile, "logfile", logfile)
	return nil
}

func StopDaemon(pidfile string) error {
	if pidfile == "" {
		return fmt.Errorf("pidfile cannot be empty")
	}

	pid, err := os.ReadFile(pidfile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Pidfile does not exist, nothing to stop", "pidfile", pidfile)
			// TODO: maybe use pkill to kill the process by name
			return nil
		}
		return fmt.Errorf("failed to read pidfile %s: %v", pidfile, err)
	}

	pidInt, err := strconv.Atoi(string(pid))
	if err != nil {
		return fmt.Errorf("invalid PID in pidfile %s: %v", pidfile, err)
	}

	// check if the process is running
	isRunning := true
	slog.Debug("Checking if daemon process is running", "pid", pidInt)
	p, err := os.FindProcess(pidInt)
	if err != nil {
		slog.Error("Failed to find process", "pid", pidInt, "error", err)
		return err
	}
	if p != nil {
		if err := p.Signal(syscall.Signal(0)); err != nil {
			slog.Warn("Process not running", "pid", pidInt, "error", err)
			isRunning = false
		}
	}

	if isRunning {
		slog.Info("Stopping daemon process", "pid", pidInt)
		if err := syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
			slog.Error("Failed to stop daemon process", "pid", pidInt, "signal", syscall.SIGTERM, "error", err)
			if err = syscall.Kill(pidInt, syscall.SIGKILL); err != nil {
				slog.Error("Failed to kill daemon process", "pid", pidInt, "signal", syscall.SIGKILL, "error", err)
				return err
			}
		}
	}

	if err := os.Remove(pidfile); err != nil {
		slog.Error("Failed to remove pidfile", "pidfile", pidfile, "error", err)
		return err
	}

	slog.Info("Daemon process stopped and pidfile removed", "pidfile", pidfile)
	return nil
}
