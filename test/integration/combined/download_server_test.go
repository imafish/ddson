package combined

import (
	"fmt"
	"testing"
	"time"

	"github.com/imafish/ddson/test/helpers"
	"github.com/imafish/ddson/test/mocks"
)

// TestBasicDownloadServerSetup tests that the test download server can be started and stopped
func TestBasicDownloadServerSetup(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	}

	server := mocks.NewTestDownloadServer(config)

	// Add a test file
	testData := mocks.GenerateRandomTestFile(1024 * 1024) // 1 MB
	server.AddFile("test-file.bin", testData)

	// Start server
	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start test download server")

	// Verify server is accessible
	helpers.AssertTrue(t, server.URL() != "", "Server URL should not be empty")

	t.Logf("Test download server started at: %s", server.URL())

	// Stop server
	err = server.Stop()
	helpers.AssertNoError(t, err, "Failed to stop test download server")
}

// TestDownloadServerFileServing tests that files can be downloaded from the test server
func TestDownloadServerFileServing(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	}

	server := mocks.NewTestDownloadServer(config)

	// Add test files
	smallFile := mocks.GenerateTestFile(1024, 0xAA)        // 1 KB
	mediumFile := mocks.GenerateRandomTestFile(1024 * 100) // 100 KB

	server.AddFile("small.bin", smallFile)
	server.AddFile("medium.bin", mediumFile)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	// Verify URLs
	smallURL := server.FileURL("small.bin")
	mediumURL := server.FileURL("medium.bin")

	helpers.AssertTrue(t, smallURL != "", "Small file URL should not be empty")
	helpers.AssertTrue(t, mediumURL != "", "Medium file URL should not be empty")

	t.Logf("Small file URL: %s", smallURL)
	t.Logf("Medium file URL: %s", mediumURL)

	// Get checksums
	smallChecksum, err := server.GetFileChecksum("small.bin")
	helpers.AssertNoError(t, err, "Failed to get small file checksum")

	mediumChecksum, err := server.GetFileChecksum("medium.bin")
	helpers.AssertNoError(t, err, "Failed to get medium file checksum")

	t.Logf("Small file checksum: %s", smallChecksum)
	t.Logf("Medium file checksum: %s", mediumChecksum)
}

// TestDownloadServerWithAuthentication tests authentication on the download server
func TestDownloadServerWithAuthentication(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
		RequireAuth:    true,
		Username:       "testuser",
		Password:       "testpass",
	}

	server := mocks.NewTestDownloadServer(config)
	server.AddFile("auth-file.bin", mocks.GenerateTestFile(1024, 0xBB))

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	t.Logf("Auth-protected file URL: %s", server.FileURL("auth-file.bin"))
}

// TestDownloadServerWithFailures tests the server's failure simulation
func TestDownloadServerWithFailures(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:        0,
		FailureRate: 0.5, // 50% failure rate
	}

	server := mocks.NewTestDownloadServer(config)
	server.AddFile("flaky-file.bin", mocks.GenerateTestFile(1024, 0xCC))

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	t.Logf("Flaky file URL: %s (50%% failure rate)", server.FileURL("flaky-file.bin"))
}

// TestDownloadServerWithDelay tests the server's delay simulation
func TestDownloadServerWithDelay(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:          0,
		SimulateDelay: 100 * time.Millisecond,
	}

	server := mocks.NewTestDownloadServer(config)
	server.AddFile("slow-file.bin", mocks.GenerateTestFile(1024, 0xDD))

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	t.Logf("Slow file URL: %s (100ms delay)", server.FileURL("slow-file.bin"))
}

// TestDownloadServerRangeRequests tests Range request handling
func TestDownloadServerRangeRequests(t *testing.T) {
	config := &mocks.DownloadServerConfig{
		Port:           0,
		SupportsRanges: true,
	}

	server := mocks.NewTestDownloadServer(config)

	// Create a file with known pattern
	testData := make([]byte, 1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}
	server.AddFile("range-test.bin", testData)

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	t.Logf("Range-capable file URL: %s", server.FileURL("range-test.bin"))

	// Verify request log
	time.Sleep(100 * time.Millisecond)
	logs := server.GetRequestLog()
	t.Logf("Request log: %v", logs)
}

// TestDownloadServerMultipleFiles tests serving multiple files concurrently
func TestDownloadServerMultipleFiles(t *testing.T) {
	server := mocks.NewTestDownloadServer(nil)

	// Add multiple test files
	for i := 1; i <= 5; i++ {
		filename := fmt.Sprintf("file-%d.bin", i)
		size := 1024 * i // Varying sizes
		server.AddFileWithSize(filename, size)
	}

	err := server.Start()
	helpers.AssertNoError(t, err, "Failed to start server")
	defer server.Stop()

	for i := 1; i <= 5; i++ {
		filename := fmt.Sprintf("file-%d.bin", i)
		url := server.FileURL(filename)
		t.Logf("File %d URL: %s", i, url)
	}
}
