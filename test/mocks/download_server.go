package mocks

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DownloadServerConfig configures the test download server behavior
type DownloadServerConfig struct {
	Port            int           // Port to listen on (0 for random)
	SupportsRanges  bool          // Whether to support HTTP Range requests
	SimulateDelay   time.Duration // Add artificial delay to responses
	FailureRate     float64       // Probability of random failure (0.0-1.0)
	RequireAuth     bool          // Whether to require authentication
	Username        string        // Expected username for auth
	Password        string        // Expected password for auth
	ConnectionDrops bool          // Randomly drop connections mid-stream
}

// TestDownloadServer is a mock HTTP server for testing downloads
type TestDownloadServer struct {
	config     *DownloadServerConfig
	httpServer *http.Server
	files      map[string][]byte
	mu         sync.RWMutex
	addr       string
	requestLog []string
}

// NewTestDownloadServer creates a new test download server
func NewTestDownloadServer(config *DownloadServerConfig) *TestDownloadServer {
	if config == nil {
		config = &DownloadServerConfig{
			Port:           0,
			SupportsRanges: true,
		}
	}

	s := &TestDownloadServer{
		config:     config,
		files:      make(map[string][]byte),
		requestLog: make([]string, 0),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDownload)

	s.httpServer = &http.Server{
		Handler: mux,
	}

	return s
}

// AddFile adds a file to the server with the given content
func (s *TestDownloadServer) AddFile(name string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[name] = data
	log.Printf("TestDownloadServer: Added file %s (%d bytes)", name, len(data))
}

// AddFileWithSize creates a file of the specified size with random content
func (s *TestDownloadServer) AddFileWithSize(name string, size int) {
	data := make([]byte, size)
	rand.Read(data)
	s.AddFile(name, data)
}

// Start starts the HTTP server
func (s *TestDownloadServer) Start() error {
	listener, err := listenOnPort(s.config.Port)
	if err != nil {
		return err
	}

	s.addr = listener.Addr().String()
	log.Printf("TestDownloadServer: Starting on %s", s.addr)

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("TestDownloadServer error: %v", err)
		}
	}()

	return nil
}

// Stop stops the HTTP server
func (s *TestDownloadServer) Stop() error {
	log.Printf("TestDownloadServer: Stopping")
	return s.httpServer.Close()
}

// URL returns the base URL of the server
func (s *TestDownloadServer) URL() string {
	return "http://" + s.addr
}

// FileURL returns the full URL for a file
func (s *TestDownloadServer) FileURL(filename string) string {
	return s.URL() + "/" + filename
}

// GetRequestLog returns the log of all requests received
func (s *TestDownloadServer) GetRequestLog() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.requestLog...)
}

// GetFileChecksum returns the SHA256 checksum of a file
func (s *TestDownloadServer) GetFileChecksum(filename string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, ok := s.files[filename]
	if !ok {
		return "", fmt.Errorf("file not found: %s", filename)
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func (s *TestDownloadServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Log request
	s.mu.Lock()
	s.requestLog = append(s.requestLog, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
	s.mu.Unlock()

	// Check authentication
	if s.config.RequireAuth {
		username, password, ok := r.BasicAuth()
		if !ok || username != s.config.Username || password != s.config.Password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Simulate random failures
	if s.config.FailureRate > 0 && rand.Float64() < s.config.FailureRate {
		statusCodes := []int{
			http.StatusInternalServerError,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		}
		status := statusCodes[rand.Intn(len(statusCodes))]
		http.Error(w, "Simulated failure", status)
		return
	}

	// Get file
	filename := strings.TrimPrefix(r.URL.Path, "/")
	s.mu.RLock()
	data, ok := s.files[filename]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Simulate delay
	if s.config.SimulateDelay > 0 {
		time.Sleep(s.config.SimulateDelay)
	}

	// Handle range requests
	var start, end int64
	start = 0
	end = int64(len(data)) - 1
	statusCode := http.StatusOK

	if s.config.SupportsRanges {
		w.Header().Set("Accept-Ranges", "bytes")

		rangeHeader := r.Header.Get("Range")
		if rangeHeader != "" {
			// Parse range header (e.g., "bytes=0-1023")
			rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
			parts := strings.Split(rangeHeader, "-")
			if len(parts) == 2 {
				if parts[0] != "" {
					start, _ = strconv.ParseInt(parts[0], 10, 64)
				}
				if parts[1] != "" {
					end, _ = strconv.ParseInt(parts[1], 10, 64)
				}
				statusCode = http.StatusPartialContent
				w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
			}
		}
	}

	// Set headers
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(statusCode)

	// Send data
	if r.Method != "HEAD" {
		content := data[start : end+1]

		// Simulate connection drops
		if s.config.ConnectionDrops && rand.Float64() < 0.3 {
			// Write partial data then close
			partialSize := len(content) / 2
			w.Write(content[:partialSize])
			// Force connection close
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
			return
		}

		w.Write(content)
	}
}

// GenerateTestFile generates a test file with predictable content
func GenerateTestFile(size int, pattern byte) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = pattern
	}
	return data
}

// GenerateRandomTestFile generates a test file with random content
func GenerateRandomTestFile(size int) []byte {
	data := make([]byte, size)
	rand.Read(data)
	return data
}

// CalculateChecksum calculates SHA256 checksum of data
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

// VerifyFileChecksum verifies the checksum of a file
func VerifyFileChecksum(filePath string, expectedChecksum string) error {
	file, err := http.Get(filePath)
	if err != nil {
		return err
	}
	defer file.Body.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file.Body); err != nil {
		return err
	}

	actualChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}
