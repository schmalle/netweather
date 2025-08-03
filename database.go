package main

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// ScanResult holds the result of a single script scan.
type ScanResult struct {
	URL         string
	ScriptURL   string
	Checksum    string
	LibraryName string
	ScannedAt   time.Time
}

// initDB initializes the database connection.
func initDB(dataSourceName string) error {
	var err error
	db, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		return err
	}
	return db.Ping()
}

// createTable creates the necessary table in the database if it doesn't exist.
func createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS scan_results (
		id INT AUTO_INCREMENT PRIMARY KEY,
		url VARCHAR(2083) NOT NULL,
		script_url VARCHAR(2083) NOT NULL,
		checksum VARCHAR(64) NOT NULL,
		library_name VARCHAR(255),
		scanned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		date DATE
	);`
	_, err := db.Exec(query)
	return err
}

// storeResult stores a scan result in the database.
func storeResult(result ScanResult) error {
	query := "INSERT INTO scan_results (url, script_url, checksum, library_name, date) VALUES (?, ?, ?, ?, ?)"
	_, err := db.Exec(query, result.URL, result.ScriptURL, result.Checksum, result.LibraryName, time.Now().Format("2006-01-02"))
	return err
}
