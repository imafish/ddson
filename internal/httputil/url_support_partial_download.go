package httputil

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

func CheckPartialDownloadSupport(url string) (bool, int64, error) {
	if url == "" {
		return false, 0, fmt.Errorf("invalid URL")
	}

	login, password, err := GetDataFromNetrc(url)
	if err != nil {
		log.Printf("Error getting credentials from .netrc: %v", err)
		return false, 0, err
	}

	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		log.Printf("Error creating HEAD request: %v", err)
		return false, 0, err
	}
	if login != "" && password != "" {
		log.Printf("Using credentials from .netrc for URL: %s", url)
		req.SetBasicAuth(login, password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error making HEAD request: %v", err)
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		log.Printf("Unexpected HTTP status: %s", resp.Status)
		return false, 0, fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	supportsPartial := resp.Header.Get("Accept-Ranges") == "bytes"
	totalSize, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		log.Printf("Error parsing Content-Length: %v", err)
		return false, 0, err
	}

	log.Printf("Supports partial download: %v, Total size: %d bytes", supportsPartial, totalSize)
	return supportsPartial, totalSize, nil
}
