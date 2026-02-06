package persistency

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/imafish/ddson/internal/database"
)

// newTestPersistency creates a Persistency backed by a temp directory + real SQLite DB.
func newTestPersistency(t *testing.T) *Persistency {
	t.Helper()
	baseDir := t.TempDir()
	p, err := NewAndInitializePersistency(baseDir)
	if err != nil {
		t.Fatalf("failed to initialize persistency: %v", err)
	}
	return p
}

// createTempFile creates a file with the given content in dir and returns its path.
func createTempFile(t *testing.T, dir, content string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "testfile-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	f.Close()
	return f.Name()
}

// --- NewAndInitializePersistency ---

func TestNewAndInitializePersistency(t *testing.T) {
	baseDir := t.TempDir()
	p, err := NewAndInitializePersistency(baseDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil Persistency")
	}

	// Verify the downloaded_files directory was created
	dirPath := filepath.Join(baseDir, downloadedFilesDir)
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("downloaded_files dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}

	// Verify the database file was created
	dbPath := filepath.Join(baseDir, downloadedFilesDB)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
}

func TestNewAndInitializePersistency_Idempotent(t *testing.T) {
	baseDir := t.TempDir()
	_, err := NewAndInitializePersistency(baseDir)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	// Call again — should not fail
	_, err = NewAndInitializePersistency(baseDir)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
}

// --- AddDownloadedFile ---

func TestAddDownloadedFile(t *testing.T) {
	p := newTestPersistency(t)

	// Create a file to "download"
	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "hello world")

	err := p.AddDownloadedFile("https://example.com/hello.txt", srcFile, "sha256-hello")
	if err != nil {
		t.Fatalf("AddDownloadedFile failed: %v", err)
	}

	// The original file should have been moved (renamed)
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("expected source file to be moved away")
	}

	// Should be retrievable
	path, err := p.GetPersistedFile("https://example.com/hello.txt", "sha256-hello")
	if err != nil {
		t.Fatalf("GetPersistedFile failed: %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty path for persisted file")
	}

	// Verify file content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read persisted file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}
}

func TestAddDownloadedFile_UpdatesExisting(t *testing.T) {
	p := newTestPersistency(t)

	// Add first version
	srcDir := t.TempDir()
	srcFile1 := createTempFile(t, srcDir, "version 1")
	err := p.AddDownloadedFile("https://example.com/data.bin", srcFile1, "sha-v1")
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	path1, _ := p.GetPersistedFile("https://example.com/data.bin", "sha-v1")
	if path1 == "" {
		t.Fatal("expected persisted file after first add")
	}

	// Add second version with different sha256
	srcFile2 := createTempFile(t, srcDir, "version 2")
	err = p.AddDownloadedFile("https://example.com/data.bin", srcFile2, "sha-v2")
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Old file should be gone from disk
	if _, err := os.Stat(path1); !os.IsNotExist(err) {
		t.Error("expected old cached file to be removed")
	}

	// New version should be retrievable
	path2, _ := p.GetPersistedFile("https://example.com/data.bin", "sha-v2")
	if path2 == "" {
		t.Fatal("expected persisted file after update")
	}

	data, _ := os.ReadFile(path2)
	if string(data) != "version 2" {
		t.Errorf("content = %q, want %q", string(data), "version 2")
	}
}

func TestAddDownloadedFile_SameSHA256UpdatesLastUsed(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	content := "unchanged content"

	srcFile1 := createTempFile(t, srcDir, content)
	err := p.AddDownloadedFile("https://example.com/stable.bin", srcFile1, "sha-stable")
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	// Add again with same URL, same sha, same size — should just update last_used
	srcFile2 := createTempFile(t, srcDir, content)
	err = p.AddDownloadedFile("https://example.com/stable.bin", srcFile2, "sha-stable")
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	// Should still be retrievable
	path, _ := p.GetPersistedFile("https://example.com/stable.bin", "sha-stable")
	if path == "" {
		t.Fatal("expected persisted file to still exist")
	}
}

func TestAddDownloadedFile_NonExistentSourceFile(t *testing.T) {
	p := newTestPersistency(t)

	err := p.AddDownloadedFile("https://example.com/ghost.bin", "/nonexistent/file", "sha")
	if err == nil {
		t.Fatal("expected error for non-existent source file")
	}
}

// --- GetPersistedFile ---

func TestGetPersistedFile_NotFound(t *testing.T) {
	p := newTestPersistency(t)

	path, err := p.GetPersistedFile("https://nonexistent.com/file", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for non-existent file, got %q", path)
	}
}

func TestGetPersistedFile_SHA256Mismatch(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "data")
	p.AddDownloadedFile("https://example.com/file.bin", srcFile, "sha-real")

	// Query with wrong sha256
	path, err := p.GetPersistedFile("https://example.com/file.bin", "sha-wrong")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path for sha256 mismatch, got %q", path)
	}
}

func TestGetPersistedFile_EmptySHA256SkipsCheck(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "any content")
	p.AddDownloadedFile("https://example.com/flexible.bin", srcFile, "sha-whatever")

	// Query with empty sha256 — should match regardless
	path, err := p.GetPersistedFile("https://example.com/flexible.bin", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path == "" {
		t.Fatal("expected match when sha256 is empty")
	}
}

func TestGetPersistedFile_FileDeletedFromDisk(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "will be deleted")
	p.AddDownloadedFile("https://example.com/vanish.bin", srcFile, "sha-vanish")

	// Get the cached path
	cachedPath, _ := p.GetPersistedFile("https://example.com/vanish.bin", "sha-vanish")
	if cachedPath == "" {
		t.Fatal("expected cached file")
	}

	// Delete the file from disk behind persistency's back
	os.Remove(cachedPath)

	// Now GetPersistedFile should return an error
	_, err := p.GetPersistedFile("https://example.com/vanish.bin", "sha-vanish")
	if err == nil {
		t.Fatal("expected error when file missing from disk")
	}
}

// --- RemoveFileAndDbEntry ---

func TestRemoveFileAndDbEntry(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "remove me")
	p.AddDownloadedFile("https://example.com/remove.bin", srcFile, "sha-rm")

	// Get the DB entry
	dbFile, _ := database.GetDownloadedFileByOriginalURL(p.db, "https://example.com/remove.bin")
	if dbFile == nil {
		t.Fatal("expected DB entry")
	}

	err := p.RemoveFileAndDbEntry(dbFile)
	if err != nil {
		t.Fatalf("RemoveFileAndDbEntry failed: %v", err)
	}

	// File should be gone from disk
	cachedPath := filepath.Join(p.baseDir, downloadedFilesDir, dbFile.Filename)
	if _, err := os.Stat(cachedPath); !os.IsNotExist(err) {
		t.Error("expected file to be removed from disk")
	}

	// DB entry should be gone
	got, _ := database.GetDownloadedFile(p.db, dbFile.Id)
	if got != nil {
		t.Error("expected DB entry to be deleted")
	}
}

func TestRemoveFileAndDbEntry_NilFile(t *testing.T) {
	p := newTestPersistency(t)

	err := p.RemoveFileAndDbEntry(nil)
	if err == nil {
		t.Fatal("expected error for nil file")
	}
}

// --- Cleanup ---

func TestCleanup_RemovesOldestWhenOverMaxSize(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()

	// Add 3 files, ~100 bytes each, with staggered last-used times
	urls := []string{
		"https://example.com/oldest.bin",
		"https://example.com/middle.bin",
		"https://example.com/newest.bin",
	}
	for i, url := range urls {
		content := make([]byte, 100)
		for j := range content {
			content[j] = byte('A' + i)
		}
		srcFile := createTempFile(t, srcDir, string(content))
		p.AddDownloadedFile(url, srcFile, "sha-"+url)

		// Manually set last_used to control ordering
		dbFile, _ := database.GetDownloadedFileByOriginalURL(p.db, url)
		dbFile.LastUsed = time.Now().Add(time.Duration(-3+i) * time.Hour)
		database.UpdateDownloadedFile(p.db, dbFile)
	}

	// Total size is ~300 bytes. Set maxSize=200 → should remove the oldest one.
	err := p.Cleanup(24*time.Hour, 100, 200)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Oldest should be gone
	path, _ := p.GetPersistedFile("https://example.com/oldest.bin", "")
	if path != "" {
		t.Error("expected oldest file to be cleaned up")
	}

	// Middle and newest should remain
	path, _ = p.GetPersistedFile("https://example.com/middle.bin", "")
	if path == "" {
		t.Error("expected middle file to remain")
	}
	path, _ = p.GetPersistedFile("https://example.com/newest.bin", "")
	if path == "" {
		t.Error("expected newest file to remain")
	}
}

func TestCleanup_RemovesOldFilesOverToleranceSize(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()

	// Add 3 files
	urls := []string{
		"https://example.com/a.bin",
		"https://example.com/b.bin",
		"https://example.com/c.bin",
	}
	for i, url := range urls {
		content := make([]byte, 100)
		srcFile := createTempFile(t, srcDir, string(content))
		p.AddDownloadedFile(url, srcFile, "sha-"+url)

		dbFile, _ := database.GetDownloadedFileByOriginalURL(p.db, url)
		// a.bin: 3 hours old (expired), b.bin: 2 hours old (expired), c.bin: recent
		dbFile.LastUsed = time.Now().Add(time.Duration(-3+i) * time.Hour)
		database.UpdateDownloadedFile(p.db, dbFile)
	}

	// Total ~300 bytes. toleranceSize=200, maxSize=1000 (won't trigger phase 1).
	// maxLife=1 hour → a.bin and b.bin are expired.
	// Phase 2: remove expired files until under toleranceSize.
	err := p.Cleanup(1*time.Hour, 200, 1000)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// a.bin should be removed (oldest expired, puts us under tolerance)
	pathA, _ := p.GetPersistedFile("https://example.com/a.bin", "")
	if pathA != "" {
		t.Error("expected a.bin to be cleaned up")
	}

	// c.bin should remain (recent)
	pathC, _ := p.GetPersistedFile("https://example.com/c.bin", "")
	if pathC == "" {
		t.Error("expected c.bin to remain")
	}
}

func TestCleanup_NothingToRemove(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()
	srcFile := createTempFile(t, srcDir, "small file")
	p.AddDownloadedFile("https://example.com/small.bin", srcFile, "sha-small")

	// Total size is tiny, well under both thresholds
	err := p.Cleanup(24*time.Hour, 1000000, 2000000)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	path, _ := p.GetPersistedFile("https://example.com/small.bin", "")
	if path == "" {
		t.Fatal("expected file to remain")
	}
}

func TestCleanup_EmptyDatabase(t *testing.T) {
	p := newTestPersistency(t)

	err := p.Cleanup(24*time.Hour, 1000, 2000)
	if err != nil {
		t.Fatalf("Cleanup on empty DB failed: %v", err)
	}
}

func TestCleanup_PreservesRecentFilesInPhase2(t *testing.T) {
	p := newTestPersistency(t)

	srcDir := t.TempDir()

	// Add 2 files, both recent
	for i := 0; i < 2; i++ {
		content := make([]byte, 100)
		srcFile := createTempFile(t, srcDir, string(content))
		url := "https://example.com/recent" + string(rune('0'+i)) + ".bin"
		p.AddDownloadedFile(url, srcFile, "sha-recent"+string(rune('0'+i)))
	}

	// Total ~200 bytes. toleranceSize=100 (triggers phase 2), maxSize=1000 (no phase 1).
	// maxLife=24h → all files are recent, so phase 2 should stop immediately.
	err := p.Cleanup(24*time.Hour, 100, 1000)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Both files should remain since they're within maxLife
	files, _ := database.GetAllDownloadedFiles(p.db)
	if len(files) != 2 {
		t.Errorf("expected 2 files to remain, got %d", len(files))
	}
}
