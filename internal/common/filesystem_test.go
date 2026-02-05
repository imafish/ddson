package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSFileSystemBasics(t *testing.T) {
	fs := OSFileSystem{}
	root, err := fs.MkdirTemp("", "ddson-fs-test-")
	if err != nil {
		t.Fatalf("MkdirTemp error: %v", err)
	}
	defer fs.RemoveAll(root)

	filePath := filepath.Join(root, "test.txt")
	f, err := fs.Create(filePath)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	_, _ = f.WriteString("hello")
	f.Close()

	info, err := fs.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if info.Size() != int64(len("hello")) {
		t.Fatalf("unexpected size: %d", info.Size())
	}

	rf, err := fs.Open(filePath)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	rf.Close()

	if err := fs.Remove(filePath); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Remove error: %v", err)
	}
}
