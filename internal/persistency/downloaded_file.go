package persistency

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sort"
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

	slog.Debug("Update last used time for persisted file", "url", url, "filename", persistedFile.Filename)
	// update last used time
	persistedFile.LastUsed = time.Now()
	err = database.UpdateDownloadedFile(p.db, persistedFile)
	if err != nil {
		slog.Warn("Failed to update last used time for downloaded file", "url", url, "filename", persistedFile.Filename, "error", err)
	}

	return fullPath, nil
}

func (p *Persistency) AddDownloadedFile(originalUrl string, downloadedFilePath string, sha256 string) error {
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

		_, err = p.createNewDownloadedFileCache(originalUrl, downloadedFilePath, fileInfo, sha256)
		if err != nil {
			slog.Error("Failed to create new downloaded file cache", "error", err)
			return err
		}
	} else {
		slog.Debug("Update only last used time", "url", originalUrl, "filename", item.Filename)
		item.LastUsed = time.Now()
		database.UpdateDownloadedFile(p.db, item)
	}

	return nil
}

// RemoveFileAndDbEntry removes the file from disk and deletes its entry from the database.
// It returns an error if the file is nil or if any operation fails.
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

// Cleanup frees up space by removing old downloaded files based on their last used time and total size.
// It removes files according to the following criteria:
// - If total size exceeds maxSize, it removes the oldest files until the size is below maxSize.
// - If total size exceeds toleranceSize, it removes files that have not been used for more than maxLife, until the size is below toleranceSize.
func (p *Persistency) Cleanup(maxLife time.Duration, toleranceSize int64, maxSize int64) error {
	slog.Info("Starting cleanup of downloaded files",
		"maxLife", maxLife,
		"toleranceSize", toleranceSize,
		"maxSize", maxSize)

	// Get all downloaded files
	files, err := database.GetAllDownloadedFiles(p.db)
	if err != nil {
		slog.Error("Failed to retrieve downloaded files from database", "error", err)
		return err
	}
	slog.Debug("Retrieved downloaded files from database", "count", len(files))

	totalSize := int64(0)
	for _, file := range files {
		filePath := path.Join(p.baseDir, downloadedFilesDir, file.Filename)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			slog.Warn("Failed to stat file", "path", filePath, "error", err)
			continue // skip this file if it doesn't exist
		}
		totalSize += fileInfo.Size()
	}
	slog.Debug("Total size of downloaded files", "size", totalSize)

	if totalSize > toleranceSize {
		slog.Debug("Should sort files for cleanup")
		sort.Slice(files, func(i, j int) bool {
			return files[i].LastUsed.Before(files[j].LastUsed)
		})
	}

	slog.Debug("Persistency cleanup: first phase.")
	for totalSize > maxSize && len(files) > 0 {
		file := files[0] // oldest file
		slog.Debug("Total size exceeds max size, removing oldest file",
			"file", file.Filename,
			"size", file.Size,
			"totalSize", totalSize,
			"maxSize", maxSize)

		err = p.RemoveFileAndDbEntry(file)
		if err != nil {
			slog.Error("Failed to remove file and database entry", "file", file.Filename, "error", err)
			return err
		}
		totalSize -= file.Size // update total size
		files = files[1:]      // remove the first file from the slice
	}
	slog.Debug("Cleanup completed, total size is now below max size",
		"totalSize", totalSize, "maxSize", maxSize)

	slog.Debug("Persistency cleanup: second phase.")
	for totalSize > toleranceSize && len(files) > 0 {
		file := files[0] // oldest file
		if time.Since(file.LastUsed) < maxLife {
			slog.Debug("File is still within max life, skipping", "file", file.Filename, "lastUsed", file.LastUsed)
			break // no more files to remove
		}

		slog.Debug("Total size exceeds tolerance size, removing old file",
			"file", file.Filename,
			"size", file.Size,
			"totalSize", totalSize,
			"toleranceSize", toleranceSize)

		err = p.RemoveFileAndDbEntry(file)
		if err != nil {
			slog.Error("Failed to remove file and database entry", "file", file.Filename, "error", err)
			return err
		}
		totalSize -= file.Size
		files = files[1:]
	}

	return nil
}

func (p *Persistency) createNewDownloadedFileCache(originalUrl string, downloadedFilePath string, fileInfo os.FileInfo, sha256 string) (*database.DownloadedFile, error) {
	// save the file to disk using a unique name
	tempFile, err := os.CreateTemp(path.Join(p.baseDir, downloadedFilesDir), "file-*")
	if err != nil {
		slog.Error("Failed to create temporary file", "error", err)
		return nil, err
	}
	tempFile.Close()

	slog.Debug("Moving downloaded file to final location", "source", downloadedFilePath, "destination", tempFile.Name())
	pathOnDisk := tempFile.Name()
	err = os.Rename(downloadedFilePath, pathOnDisk)
	if err != nil {
		slog.Error("Failed to move downloaded file to final location", "source", downloadedFilePath, "destination", pathOnDisk, "error", err)
		return nil, err
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
		return nil, err
	}

	slog.Info("New downloaded file cache created",
		"url", originalUrl,
		"filename", filename,
		"size", fileInfo.Size(),
		"sha256", sha256)
	return &downloadedFile, nil
}
