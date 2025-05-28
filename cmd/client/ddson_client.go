package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"

	"internal/common"
	"internal/version"
)

var (
	addr        = flag.String("addr", "localhost:5510", "the address to connect to")
	clientName  = flag.String("name", "", "the name of the client")
	downloadUrl = flag.String("url", "", "URL to download from")
	output      = flag.String("output", "", "output file name")
	servicePort = flag.Int("port", 5510, "the port to listen on")
	debug       = flag.Bool("debug", false, "enable debug mode (default: false)")
	sha256      = flag.String("sha256", "", "SHA256 checksum of the file to download (optional, for verification)")
	daemonize   = flag.Bool("daemon", false, "run as a daemon process (default: false)")
	forceDaemon = flag.Bool("force", false, "force daemonize even if pidfile exists (default: false)")
	stopDaemon  = flag.Bool("stop", false, "stop the daemon process (default: false)")
)

const (
	pidfile = "/var/run/ddson.pid"
	logfile = "/var/log/ddson.log"
)

func main() {
	flag.Parse()

	// Set up slog logger
	var logger *slog.Logger
	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	slog.SetDefault(logger)

	slog.Info("Starting ddson client", "args", os.Args, "version", version.VersionString)

	switch {
	case *stopDaemon:
		slog.Info("Stopping daemon process", "pidfile", pidfile)
		err := doStopDaemon()
		if err != nil {
			slog.Error("Failed to stop daemon process", "error", err)
			os.Exit(1)
		}
		return
	case *daemonize:
		slog.Info("Daemonizing process", "pidfile", pidfile, "logfile", logfile)
		err := doDaemonize(*forceDaemon)
		if err != nil {
			slog.Error("Failed to daemonize process", "error", err)
			os.Exit(1)
		}
		return
	}

	// TODO: include both mode in the same process
	if *downloadUrl != "" {
		// downloader mode
		if *output == "" {
			parsedURL, err := url.Parse(*downloadUrl)
			if err != nil {
				slog.Error("failed to parse URL", "error", err)
				os.Exit(1)
			}

			pathSegments := parsedURL.Path
			*output = path.Base(pathSegments)
			slog.Debug("Extracted file name from URL", "fileName", *output)
		}

		slog.Info("Downloading", "from", *downloadUrl, "to", *output)
		download()

	} else {

		// client agent mode
		if *clientName == "" {
			hostname, err := os.Hostname()
			if err != nil {
				slog.Error("failed to get hostname", "error", err)
				os.Exit(1)
			}
			*clientName = hostname
		}

		slog.Info("Starting agent mode", "clientName", *clientName, "version", version.VersionString)
		slog.Debug("Server address", "addr", *addr)

		runAgent()
	}
}

func doStopDaemon() error {
	return common.StopDaemon(pidfile)
}

func doDaemonize(force bool) error {
	if force {
		slog.Info("Forcing daemonization, stop existing daemon if running")
		err := doStopDaemon()
		if err != nil {
			slog.Error("Failed to stop existing daemon process", "error", err)
			return err
		}
	} else if _, err := os.Stat(pidfile); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("pidfile %s already exists, use --force to overwrite or stop the existing daemon first", pidfile)
	}

	err := common.Daemonize(pidfile, logfile)
	if err != nil {
		return err
	}

	slog.Info("Daemon process started successfully", "pidfile", pidfile, "logfile", logfile)
	return nil
}
