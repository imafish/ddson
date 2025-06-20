package database

import (
	"database/sql"
	"time"

	"log"

	_ "github.com/mattn/go-sqlite3"
)

// DownloadedFile represents a file downloaded and stored in the database.
type DownloadedFile struct {
	Id          int64     `db:"id"`
	OriginalURL string    `db:"original_url"`
	LastUsed    time.Time `db:"last_used"`
	Size        int64     `db:"size"`
	SHA256      string    `db:"sha256"`
	PathOnDisk  string    `db:"path_on_disk"`
	Created     time.Time `db:"created"`
}

// CreateTable creates the downloaded_files table if it does not exist.
func CreateTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS downloaded_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_url TEXT NOT NULL,
		size INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		path_on_disk TEXT NOT NULL,
		last_used DATETIME NOT NULL,
		created DATETIME NOT NULL
	);`

	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to create table: %v", err)
		return err
	}

	return nil
}

// GetAllDownloadedFiles retrieves all DownloadedFile entries from the database.
func GetAllDownloadedFiles(db *sql.DB) ([]DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, path_on_disk, last_used, created
	FROM downloaded_files;`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Failed to retrieve all downloaded files: %v", err)
		return nil, err
	}
	defer rows.Close()

	var files []DownloadedFile
	for rows.Next() {
		var file DownloadedFile
		err := rows.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.PathOnDisk, &file.LastUsed, &file.Created)
		if err != nil {
			log.Printf("Failed to scan row: %v", err)
			return nil, err
		}
		files = append(files, file)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating rows: %v", err)
		return nil, err
	}

	return files, nil
}

// GetDownloadedFileByOriginalURL retrieves a DownloadedFile by its original URL.
func GetDownloadedFileByOriginalURL(db *sql.DB, originalURL string) (*DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, path_on_disk, last_used, created
	FROM downloaded_files
	WHERE original_url = ?;`

	row := db.QueryRow(query, originalURL)

	var file DownloadedFile
	err := row.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.PathOnDisk, &file.LastUsed, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		log.Printf("Failed to retrieve downloaded file by original URL: %v", err)
		return nil, err
	}

	return &file, nil
}

// InsertDownloadedFile inserts a new DownloadedFile into the database.
func InsertDownloadedFile(db *sql.DB, file *DownloadedFile) (int64, error) {
	query := `
	INSERT INTO downloaded_files (original_url, size, sha256, path_on_disk, last_used, created)
	VALUES (?, ?, ?, ?, ?, ?);`

	created := time.Now()
	result, err := db.Exec(query, file.OriginalURL, file.Size, file.SHA256, file.PathOnDisk, file.LastUsed, created)
	if err != nil {
		log.Printf("Failed to insert downloaded file: %v", err)
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Failed to retrieve last insert ID: %v", err)
		return 0, err
	}

	file.Created = created
	file.Id = id

	return id, nil
}

// GetDownloadedFile retrieves a DownloadedFile by its ID.
func GetDownloadedFile(db *sql.DB, id int64) (*DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, path_on_disk, last_used, created
	FROM downloaded_files
	WHERE id = ?;`

	row := db.QueryRow(query, id)

	var file DownloadedFile
	err := row.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.PathOnDisk, &file.LastUsed, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		log.Printf("Failed to retrieve downloaded file: %v", err)
		return nil, err
	}

	return &file, nil
}

// UpdateDownloadedFile updates an existing DownloadedFile in the database.
func UpdateDownloadedFile(db *sql.DB, file *DownloadedFile) error {
	query := `
	UPDATE downloaded_files
	SET original_url = ?, size = ?, sha256 = ?, path_on_disk = ?, last_used = ?
	WHERE id = ?;`

	_, err := db.Exec(query, file.OriginalURL, file.Size, file.SHA256, file.PathOnDisk, file.LastUsed, file.Id)
	if err != nil {
		log.Printf("Failed to update downloaded file: %v", err)
		return err
	}

	return nil
}

// DeleteDownloadedFile removes a DownloadedFile from the database by its ID.
func DeleteDownloadedFile(db *sql.DB, id int64) error {
	query := `
	DELETE FROM downloaded_files
	WHERE id = ?;`

	_, err := db.Exec(query, id)
	if err != nil {
		log.Printf("Failed to delete downloaded file: %v", err)
		return err
	}

	return nil
}
