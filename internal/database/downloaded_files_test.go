package database

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Fatalf("database not reachable: %v", err)
	}
}

func TestCreateTable_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer db.Close()

	if err := CreateTable(db); err != nil {
		t.Fatalf("first CreateTable failed: %v", err)
	}
	if err := CreateTable(db); err != nil {
		t.Fatalf("second CreateTable failed: %v", err)
	}
}

func TestInsertAndGetDownloadedFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	now := time.Now().Truncate(time.Second)
	file := &DownloadedFile{
		OriginalURL: "https://example.com/file.tar.gz",
		Filename:    "file-abc123",
		Size:        1024,
		SHA256:      "deadbeef",
		LastUsed:    now,
	}

	id, err := InsertDownloadedFile(db, file)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}
	if file.Id != id {
		t.Errorf("file.Id = %d, want %d", file.Id, id)
	}

	got, err := GetDownloadedFile(db, id)
	if err != nil {
		t.Fatalf("GetDownloadedFile failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.OriginalURL != file.OriginalURL {
		t.Errorf("OriginalURL = %q, want %q", got.OriginalURL, file.OriginalURL)
	}
	if got.Filename != file.Filename {
		t.Errorf("Filename = %q, want %q", got.Filename, file.Filename)
	}
	if got.Size != file.Size {
		t.Errorf("Size = %d, want %d", got.Size, file.Size)
	}
	if got.SHA256 != file.SHA256 {
		t.Errorf("SHA256 = %q, want %q", got.SHA256, file.SHA256)
	}
}

func TestGetDownloadedFile_NotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	got, err := GetDownloadedFile(db, 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent id, got %+v", got)
	}
}

func TestGetDownloadedFileByOriginalURL(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	file := &DownloadedFile{
		OriginalURL: "https://example.com/data.zip",
		Filename:    "data-xyz",
		Size:        2048,
		SHA256:      "abcdef",
		LastUsed:    time.Now(),
	}
	InsertDownloadedFile(db, file)

	got, err := GetDownloadedFileByOriginalURL(db, "https://example.com/data.zip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.OriginalURL != file.OriginalURL {
		t.Errorf("OriginalURL = %q, want %q", got.OriginalURL, file.OriginalURL)
	}
}

func TestGetDownloadedFileByOriginalURL_NotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	got, err := GetDownloadedFileByOriginalURL(db, "https://nonexistent.com/file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestUpdateDownloadedFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	file := &DownloadedFile{
		OriginalURL: "https://example.com/old.tar",
		Filename:    "old-file",
		Size:        100,
		SHA256:      "sha-old",
		LastUsed:    time.Now().Add(-24 * time.Hour),
	}
	InsertDownloadedFile(db, file)

	file.Size = 200
	file.SHA256 = "sha-new"
	file.LastUsed = time.Now()

	err = UpdateDownloadedFile(db, file)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, _ := GetDownloadedFile(db, file.Id)
	if got.Size != 200 {
		t.Errorf("Size = %d, want 200", got.Size)
	}
	if got.SHA256 != "sha-new" {
		t.Errorf("SHA256 = %q, want %q", got.SHA256, "sha-new")
	}
}

func TestDeleteDownloadedFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	file := &DownloadedFile{
		OriginalURL: "https://example.com/delete-me.tar",
		Filename:    "delete-me",
		Size:        500,
		SHA256:      "sha-del",
		LastUsed:    time.Now(),
	}
	InsertDownloadedFile(db, file)

	err = DeleteDownloadedFile(db, file.Id)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	got, _ := GetDownloadedFile(db, file.Id)
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestGetAllDownloadedFiles(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	// Empty table
	files, err := GetAllDownloadedFiles(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}

	// Insert 3 files
	for i := 0; i < 3; i++ {
		f := &DownloadedFile{
			OriginalURL: "https://example.com/file" + string(rune('A'+i)),
			Filename:    "file-" + string(rune('A'+i)),
			Size:        int64((i + 1) * 100),
			SHA256:      "sha-" + string(rune('A'+i)),
			LastUsed:    time.Now(),
		}
		InsertDownloadedFile(db, f)
	}

	files, err = GetAllDownloadedFiles(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestDeleteDownloadedFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := LoadDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to load database: %v", err)
	}
	defer db.Close()
	CreateTable(db)

	err = DeleteDownloadedFile(db, 999)
	if err != nil {
		t.Fatalf("unexpected error deleting non-existent: %v", err)
	}
}
