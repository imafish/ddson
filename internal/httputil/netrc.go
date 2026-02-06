package httputil

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path"

	"github.com/bgentry/go-netrc/netrc"

	"github.com/imafish/ddson/internal/common"
)

// GetCredentialsFromNetrcFile looks up login and password for the given URL's host
// in the specified .netrc file. If the file does not exist or contains no matching
// entry, empty strings are returned without error.
func GetCredentialsFromNetrcFile(downloadUrl string, netrcPath string) (string, string, error) {
	parsedURL, err := url.Parse(downloadUrl)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	stat, err := os.Stat(netrcPath)
	if err != nil || stat.IsDir() {
		slog.Debug("No .netrc file found or it is a directory", "path", netrcPath)
		return "", "", nil
	}

	nrc, err := netrc.ParseFile(netrcPath)
	if err != nil {
		slog.Error("Failed to parse .netrc file", "path", netrcPath, "error", err)
		return "", "", err
	}

	machine := nrc.FindMachine(parsedURL.Host)
	if machine == nil {
		slog.Debug("No machine entry found in .netrc for host", "host", parsedURL.Host)
		return "", "", nil
	}
	return machine.Login, machine.Password, nil
}

// GetDataFromNetrc looks up login and password for the given URL in the user's
// default .netrc file (~/.netrc). This is a convenience wrapper around
// GetCredentialsFromNetrcFile.
func GetDataFromNetrc(downloadUrl string) (string, string, error) {
	homeDir, err := common.OriginalUserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	netrcPath := path.Join(homeDir, ".netrc")
	return GetCredentialsFromNetrcFile(downloadUrl, netrcPath)
}
