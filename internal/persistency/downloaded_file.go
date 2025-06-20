package persistency

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"internal/database"
)

type Persistency struct {
	baseDir string
	db      *sql.DB
}

func NewPersistency(baseDir string) (*Persistency, error) {
	d, err := database.LoadDatabase(baseDir + "/downloaded_files.db")
	if err != nil {
		return nil, err
	}

	// will not create table if exist
	err = database.CreateTable(d)
	if err != nil {
		return nil, err
	}

	return &Persistency{
		baseDir: baseDir,
		db:      d,
	}, nil
}

func (p *Persistency) GetPersistedFile(url string, sha256 string) (string, error) {
	// search db for item with the same url.
	// if sha256 is provided, also check if it matches
	persistedFile, err := database.GetDownloadedFileByOriginalURL(p.db, url)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // no file found
		}
		slog.Error("Failed to get downloaded file by original URL", "url", url, "error", err)
		return "", err
	}
	if sha256 != "" && persistedFile.SHA256 != sha256 {
		slog.Warn("SHA256 mismatch for persisted file", "url", url, "expected_sha256", sha256, "actual_sha256", persistedFile.SHA256)
		return "", fmt.Errorf("SHA256 mismatch for persisted file: expected %s, got %s", sha256, persistedFile.SHA256)
	}

	return persistedFile.PathOnDisk, nil
}

func (p *Persistency) NewDownloadedFile(originalUrl string, file *os.File, sha256 string) error {
	// first check if there is already a item with the same url
	item, err := database.GetDownloadedFileByOriginalURL(p.db, originalUrl)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	fileInfo, err := file.Stat()
	if err != nil {
		slog.Error("Failed to get file info", "error", err)
		return err
	}

	shouldUpdate := false
	if item == nil {
		shouldUpdate = true
	} else {
		if (sha256 != "" && item.SHA256 != sha256) || item.Size != fileInfo.Size() {
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		err = p.RemoveFileAndDbEntry(item)
		if err != nil {
			slog.Error("Failed to remove existing file and database entry", "error", err)
			return err
		}

		err = p.createNewDownloadedFileCache(originalUrl, file, fileInfo, sha256)
		if err != nil {
			slog.Error("Failed to create new downloaded file cache", "error", err)
			return err
		}
	}
	return nil
}

func (p *Persistency) RemoveFileAndDbEntry(file *database.DownloadedFile) error {
	if file == nil {
		return fmt.Errorf("file is nil, cannot remove")
	}

	// remove file from disk
	err := os.Remove(file.PathOnDisk)
	if err != nil {
		slog.Error("Failed to remove file", "path", file.PathOnDisk, "error", err)
		return err
	}

	// remove entry from database
	err = database.DeleteDownloadedFile(p.db, file.Id)
	if err != nil {
		slog.Error("Failed to delete downloaded file entry", "id", file.Id, "error", err)
		return err
	}

	return nil
}

func (p *Persistency) createNewDownloadedFileCache(originalUrl string, file *os.File, fileInfo os.FileInfo, sha256 string) error {
	// save the file to disk using a unique name
	tempFile, err := os.CreateTemp(p.baseDir+"/downloaded_files", "file-*")
	if err != nil {
		slog.Error("Failed to create temporary file", "error", err)
		return err
	}
	defer tempFile.Close()

	pathOnDisk := tempFile.Name()
	_, err = file.Seek(0, 0) // Reset file pointer to the beginning
	if err != nil {
		slog.Error("Failed to reset file pointer", "error", err)
		return err
	}

	_, err = io.Copy(tempFile, file)
	if err != nil {
		slog.Error("Failed to copy file contents", "error", err)
		return err
	}

	downloadedFile := database.DownloadedFile{
		OriginalURL: originalUrl,
		PathOnDisk:  pathOnDisk,
		Size:        fileInfo.Size(),
		SHA256:      sha256,
		LastUsed:    time.Now(),
	}

	_, err = database.InsertDownloadedFile(p.db, &downloadedFile)
	if err != nil {
		slog.Error("Failed to insert downloaded file entry", "error", err)
		return err
	}

	return nil
}
