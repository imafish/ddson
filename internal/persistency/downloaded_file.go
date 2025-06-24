package persistency

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path"
	"time"

	"internal/database"
)

type Persistency struct {
	baseDir string
	db      *sql.DB
}

const downloadedFilesDir = "downloaded_files"
const downloadedFilesDB = "downloaded_files.db"

func NewAndInitializePersistency(baseDir string) (*Persistency, error) {
	err := os.MkdirAll(path.Join(baseDir, downloadedFilesDir), 0755)
	if err != nil {
		slog.Error("Failed to create downloaded files directory", "path", path.Join(baseDir, downloadedFilesDir), "error", err)
		return nil, fmt.Errorf("failed to create downloaded files directory: %w", err)
	}

	d, err := database.LoadDatabase(path.Join(baseDir, downloadedFilesDB))
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
		slog.Error("Failed to get downloaded file by original URL", "url", url, "error", err)
	}
	if persistedFile == nil {
		slog.Debug("No persisted file found for URL", "url", url)
		return "", nil
	}
	if sha256 != "" && persistedFile.SHA256 != sha256 {
		slog.Warn("SHA256 mismatch for persisted file", "url", url, "expected_sha256", sha256, "actual_sha256", persistedFile.SHA256)
		return "", nil
	}

	// check if the file exists on disk
	fullPath := path.Join(p.baseDir, downloadedFilesDir, persistedFile.Filename)
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("file does not exist on disk: %w", err)
	}

	// TODO: skip verifying the file checksum for now

	return fullPath, nil
}

func (p *Persistency) NewDownloadedFile(originalUrl string, downloadedFilePath string, sha256 string) error {
	file, err := os.Open(downloadedFilePath)
	if err != nil {
		slog.Error("Failed to open downloaded file", "path", downloadedFilePath, "error", err)
		return err
	}
	fileInfo, err := file.Stat()
	file.Close()
	if err != nil {
		slog.Error("Failed to get file info", "error", err)
		return err
	}

	shouldUpdate := false
	item, _ := database.GetDownloadedFileByOriginalURL(p.db, originalUrl)
	if item == nil {
		shouldUpdate = true
	} else {
		if (sha256 != "" && item.SHA256 != sha256) || item.Size != fileInfo.Size() {
			shouldUpdate = true
		}
	}

	if shouldUpdate {
		if item != nil {
			slog.Info("Updating existing downloaded file cache", "url", originalUrl, "existingPath", item.Filename, "newPath", downloadedFilePath)
			err = p.RemoveFileAndDbEntry(item)
			if err != nil {
				slog.Error("Failed to remove existing file and database entry", "error", err)
				return err
			}
		}

		err = p.createNewDownloadedFileCache(originalUrl, downloadedFilePath, fileInfo, sha256)
		if err != nil {
			slog.Error("Failed to create new downloaded file cache", "error", err)
			return err
		}
	} else {
		slog.Debug("No update needed for downloaded file cache", "url", originalUrl, "filename", item.Filename)
	}

	return nil
}

func (p *Persistency) RemoveFileAndDbEntry(file *database.DownloadedFile) error {
	if file == nil {
		return fmt.Errorf("file is nil, cannot remove")
	}

	absolutePath := path.Join(p.baseDir, downloadedFilesDir, file.Filename)
	// remove file from disk
	err := os.Remove(absolutePath)
	if err != nil {
		slog.Error("Failed to remove file", "path", absolutePath, "error", err)
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

func (p *Persistency) createNewDownloadedFileCache(originalUrl string, downloadedFilePath string, fileInfo os.FileInfo, sha256 string) error {
	// save the file to disk using a unique name
	tempFile, err := os.CreateTemp(path.Join(p.baseDir, downloadedFilesDir), "file-*")
	if err != nil {
		slog.Error("Failed to create temporary file", "error", err)
		return err
	}
	tempFile.Close()

	slog.Debug("Moving downloaded file to final location", "source", downloadedFilePath, "destination", tempFile.Name())
	pathOnDisk := tempFile.Name()
	err = os.Rename(downloadedFilePath, pathOnDisk)
	if err != nil {
		slog.Error("Failed to move downloaded file to final location", "source", downloadedFilePath, "destination", pathOnDisk, "error", err)
		return err
	}

	stat2, err := os.Stat(pathOnDisk)
	if err != nil {
		slog.Error("Failed to stat moved file", "path", pathOnDisk, "error", err)
	} else {
		slog.Debug("Moved file info", "path", pathOnDisk, "size", stat2.Size(), "mode", stat2.Mode(), "modTime", stat2.ModTime())
	}

	filename := path.Base(pathOnDisk)

	downloadedFile := database.DownloadedFile{
		OriginalURL: originalUrl,
		Filename:    filename,
		Size:        fileInfo.Size(),
		SHA256:      sha256,
		LastUsed:    time.Now(),
	}

	_, err = database.InsertDownloadedFile(p.db, &downloadedFile)
	if err != nil {
		slog.Error("Failed to insert downloaded file entry", "error", err)
		return err
	}

	slog.Info("New downloaded file cache created",
		"url", originalUrl,
		"filename", filename,
		"size", fileInfo.Size(),
		"sha256", sha256)
	return nil
}
