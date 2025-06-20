package httputil

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"

	"github.com/bgentry/go-netrc/netrc"

	"internal/common"
)

func GetDataFromNetrc(downloadUrl string) (string, string, error) {
	// Parse URL
	parsedURL, err := url.Parse(downloadUrl)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	// Get path of .netrc file
	homeDir, err := common.OriginalUserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	// TODO: allow user to specify .netrc path
	netrcPath := path.Join(homeDir, ".netrc")

	// Parse .netrc
	if stat, err := os.Stat(netrcPath); err == nil && !stat.IsDir() {
		nrc, err := netrc.ParseFile(netrcPath)
		if err != nil {
			slog.Error("Failed to parse .netrc file", "path", netrcPath, "error", err)
			return "", "", err
		}

		// Find machine entry
		machine := nrc.FindMachine(parsedURL.Host)
		if machine == nil {
			slog.Debug("No machine entry found in .netrc for host", "host", parsedURL.Host)
			return "", "", nil
		}
		return machine.Login, machine.Password, nil
	}

	slog.Debug("No .netrc file found or it is a directory", "path", netrcPath)
	return "", "", nil
}
