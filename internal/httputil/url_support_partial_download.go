package httputil

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

// CheckPartialDownloadSupport sends a HEAD request to the given URL and checks
// whether the server supports partial (range) downloads. It uses the provided
// HTTP client and credential provider. Returns whether partial downloads are
// supported, the total content size, and any error encountered.
func CheckPartialDownloadSupport(url string, client HTTPDoer, login, password string) (bool, int64, error) {
	if url == "" {
		return false, 0, fmt.Errorf("invalid URL")
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		slog.Error("Error creating HEAD request", "error", err)
		return false, 0, err
	}
	if login != "" && password != "" {
		slog.Info("Using credentials for URL", "url", url)
		req.SetBasicAuth(login, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error making HEAD request", "error", err)
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		slog.Warn("Unexpected HTTP status", "status", resp.Status)
		return false, 0, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	supportsPartial := resp.Header.Get("Accept-Ranges") == "bytes"
	totalSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		slog.Error("Error parsing Content-Length", "error", err)
		return false, 0, err
	}

	slog.Info("Partial download check complete", "supportsPartial", supportsPartial, "totalSize", totalSize)
	return supportsPartial, totalSize, nil
}
