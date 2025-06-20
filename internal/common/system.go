package common

import (
	"fmt"
	"log/slog"
	"os"
	"os/user"
)

func OriginalUserHomeDir() (string, error) {
	// First try SUDO_USER environment variable
	username := os.Getenv("SUDO_USER")
	if username == "" {
		// Not running under sudo, get current user
		currentUser, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to get current user: %v", err)
		}
		return currentUser.HomeDir, nil
	} else {
		slog.Debug("Running under sudo, using SUDO_USER", "username", username)
	}

	// Lookup the original user
	originalUser, err := user.Lookup(username)
	if err != nil {
		homeDir := "/home/" + username // Fallback to default home directory
		slog.Warn("Failed to lookup original user, using default", "username", username, "error", err, "default value", homeDir)
		return homeDir, nil
	}
	return originalUser.HomeDir, nil
}
