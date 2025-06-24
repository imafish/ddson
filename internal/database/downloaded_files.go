package database

import (
	"database/sql"
	"log/slog"
	"time"

	"log"

	// _ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite" // Use modernc.org/sqlite for better compatibility
)

// DownloadedFile represents a file downloaded and stored in the database.
type DownloadedFile struct {
	Id          int64     `db:"id"`
	OriginalURL string    `db:"original_url"`
	LastUsed    time.Time `db:"last_used"`
	Size        int64     `db:"size"`
	SHA256      string    `db:"sha256"`
	Filename    string    `db:"filename"`
	Created     time.Time `db:"created"`
}

// CreateTable creates the downloaded_files table if it does not exist.
//
// Input:
//
//	db - a pointer to an open sql.DB connection.
//
// Returns:
//
//	error - non-nil if the table creation fails, otherwise nil.
func CreateTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS downloaded_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_url TEXT NOT NULL,
		size INTEGER NOT NULL,
		sha256 TEXT NOT NULL,
		filename TEXT NOT NULL,
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
//
// Input:
//
//	db - a pointer to an open sql.DB connection.
//
// Returns:
//
//	[]DownloadedFile - a slice of all DownloadedFile records found.
//	error            - non-nil if the query or scan fails, otherwise nil.
func GetAllDownloadedFiles(db *sql.DB) ([]DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, filename, last_used, created
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
		err := rows.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.Filename, &file.LastUsed, &file.Created)
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
//
// Input:
//
//	db          - a pointer to an open sql.DB connection.
//	originalURL - the original URL string to search for.
//
// Returns:
//
//	*DownloadedFile - pointer to the found DownloadedFile, or nil if not found.
//	error           - non-nil if the query or scan fails, otherwise nil.
func GetDownloadedFileByOriginalURL(db *sql.DB, originalURL string) (*DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, filename, last_used, created
	FROM downloaded_files
	WHERE original_url = ?;`

	row := db.QueryRow(query, originalURL)

	var file DownloadedFile
	err := row.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.Filename, &file.LastUsed, &file.Created)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No file found with the given URL
		}
		slog.Error("Failed to retrieve downloaded file by original URL", "url", originalURL, "error", err)
		return nil, err
	}

	return &file, nil
}

// InsertDownloadedFile inserts a new DownloadedFile into the database.
//
// Input:
//
//	db   - a pointer to an open sql.DB connection.
//	file - pointer to a DownloadedFile struct to insert (Id and Created will be set).
//
// Returns:
//
//	int64 - the ID of the newly inserted record.
//	error - non-nil if the insert fails, otherwise nil.
func InsertDownloadedFile(db *sql.DB, file *DownloadedFile) (int64, error) {
	query := `
	INSERT INTO downloaded_files (original_url, size, sha256, filename, last_used, created)
	VALUES (?, ?, ?, ?, ?, ?);`

	created := time.Now()
	result, err := db.Exec(query, file.OriginalURL, file.Size, file.SHA256, file.Filename, file.LastUsed, created)
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
//
// Input:
//
//	db - a pointer to an open sql.DB connection.
//	id - the ID of the DownloadedFile to retrieve.
//
// Returns:
//
//	*DownloadedFile - pointer to the found DownloadedFile, or nil if not found.
//	error           - non-nil if the query or scan fails, otherwise nil.
func GetDownloadedFile(db *sql.DB, id int64) (*DownloadedFile, error) {
	query := `
	SELECT id, original_url, size, sha256, filename, last_used, created
	FROM downloaded_files
	WHERE id = ?;`

	row := db.QueryRow(query, id)

	var file DownloadedFile
	err := row.Scan(&file.Id, &file.OriginalURL, &file.Size, &file.SHA256, &file.Filename, &file.LastUsed, &file.Created)
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
//
// Input:
//
//	db   - a pointer to an open sql.DB connection.
//	file - pointer to a DownloadedFile struct with updated fields (must include valid Id).
//
// Returns:
//
//	error - non-nil if the update fails, otherwise nil.
func UpdateDownloadedFile(db *sql.DB, file *DownloadedFile) error {
	query := `
	UPDATE downloaded_files
	SET original_url = ?, size = ?, sha256 = ?, filename = ?, last_used = ?
	WHERE id = ?;`

	_, err := db.Exec(query, file.OriginalURL, file.Size, file.SHA256, file.Filename, file.LastUsed, file.Id)
	if err != nil {
		log.Printf("Failed to update downloaded file: %v", err)
		return err
	}

	return nil
}

// DeleteDownloadedFile removes a DownloadedFile from the database by its ID.
//
// Input:
//
//	db - a pointer to an open sql.DB connection.
//	id - the ID of the DownloadedFile to delete.
//
// Returns:
//
//	error - non-nil if the deletion fails, otherwise nil.
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
