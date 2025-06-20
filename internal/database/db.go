package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// LoadDatabase initializes a connection to the SQLite database.
func LoadDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Failed to open database: %v", err)
		return nil, err
	}

	// Verify the connection is valid
	if err := db.Ping(); err != nil {
		log.Printf("Failed to ping database: %v", err)
		return nil, err
	}

	return db, nil
}
