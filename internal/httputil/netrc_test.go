package httputil

import (
	"os"
	"path/filepath"
	"testing"
)

func writeNetrc(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, ".netrc")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test .netrc: %v", err)
	}
	return p
}

func TestGetCredentialsFromNetrcFile_MatchingHost(t *testing.T) {
	dir := t.TempDir()
	path := writeNetrc(t, dir, "machine example.com\nlogin myuser\npassword mypass\n")

	login, pass, err := GetCredentialsFromNetrcFile("https://example.com/file.tar.gz", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "myuser" {
		t.Errorf("login = %q, want %q", login, "myuser")
	}
	if pass != "mypass" {
		t.Errorf("password = %q, want %q", pass, "mypass")
	}
}

func TestGetCredentialsFromNetrcFile_NoMatchingHost(t *testing.T) {
	dir := t.TempDir()
	path := writeNetrc(t, dir, "machine other.com\nlogin user\npassword pass\n")

	login, pass, err := GetCredentialsFromNetrcFile("https://example.com/file.tar.gz", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "" || pass != "" {
		t.Errorf("expected empty credentials, got login=%q password=%q", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	content := `machine alpha.com
login alpha_user
password alpha_pass

machine beta.com
login beta_user
password beta_pass
`
	path := writeNetrc(t, dir, content)

	login, pass, err := GetCredentialsFromNetrcFile("https://beta.com/download", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "beta_user" || pass != "beta_pass" {
		t.Errorf("got login=%q password=%q, want beta_user/beta_pass", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_FileDoesNotExist(t *testing.T) {
	login, pass, err := GetCredentialsFromNetrcFile("https://example.com/file", "/nonexistent/.netrc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "" || pass != "" {
		t.Errorf("expected empty credentials when .netrc doesn't exist, got login=%q password=%q", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_PathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	// Pass a directory path instead of a file
	login, pass, err := GetCredentialsFromNetrcFile("https://example.com/file", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "" || pass != "" {
		t.Errorf("expected empty credentials when path is directory, got login=%q password=%q", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_InvalidURL(t *testing.T) {
	dir := t.TempDir()
	path := writeNetrc(t, dir, "machine example.com\nlogin user\npassword pass\n")

	_, _, err := GetCredentialsFromNetrcFile("://invalid-url", path)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestGetCredentialsFromNetrcFile_MalformedNetrc(t *testing.T) {
	dir := t.TempDir()
	// Write malformed content — go-netrc may or may not error on this,
	// but the function should not panic.
	path := writeNetrc(t, dir, "this is not a valid netrc format {{{}}}}")

	// We just verify it doesn't panic; error behavior depends on the parser
	_, _, _ = GetCredentialsFromNetrcFile("https://example.com/file", path)
}

func TestGetCredentialsFromNetrcFile_URLWithPort(t *testing.T) {
	dir := t.TempDir()
	path := writeNetrc(t, dir, "machine example.com:8443\nlogin portuser\npassword portpass\n")

	login, pass, err := GetCredentialsFromNetrcFile("https://example.com:8443/file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "portuser" || pass != "portpass" {
		t.Errorf("got login=%q password=%q, want portuser/portpass", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_DefaultEntry(t *testing.T) {
	dir := t.TempDir()
	content := "default\nlogin defaultuser\npassword defaultpass\n"
	path := writeNetrc(t, dir, content)

	login, pass, err := GetCredentialsFromNetrcFile("https://anything.com/file", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// go-netrc should match the "default" entry for any host
	if login != "defaultuser" || pass != "defaultpass" {
		t.Errorf("got login=%q password=%q, want defaultuser/defaultpass", login, pass)
	}
}

func TestGetCredentialsFromNetrcFile_EmptyURL(t *testing.T) {
	dir := t.TempDir()
	path := writeNetrc(t, dir, "machine example.com\nlogin user\npassword pass\n")

	// Empty URL parses to empty host — should not match "example.com"
	login, pass, err := GetCredentialsFromNetrcFile("", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "" || pass != "" {
		t.Errorf("expected empty credentials for empty URL, got login=%q password=%q", login, pass)
	}
}
