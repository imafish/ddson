package database

import (
	"database/sql"
	"fmt"
	"log"

	// _ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite" // Use modernc.org/sqlite for better compatibility
)

// LoadDatabase initializes a connection to the SQLite database.
func LoadDatabase(dbPath string) (*sql.DB, error) {
	command := fmt.Sprintf("file:%s", dbPath)
	db, err := sql.Open("sqlite", command)
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
