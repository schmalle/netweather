package main

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// ScanResult holds the result of a single script scan.
type ScanResult struct {
	URL              string
	ScriptURL        string
	Checksum         string
	LibraryName      string
	LibraryVersion   string
	IdentifiedBy     string // Method used for identification (url-pattern, api, code-analysis, etc.)
	ScannedAt        time.Time
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
	// Create scan_results table
	query := `
	CREATE TABLE IF NOT EXISTS scan_results (
		id INT AUTO_INCREMENT PRIMARY KEY,
		url VARCHAR(2083) NOT NULL,
		script_url VARCHAR(2083) NOT NULL,
		checksum VARCHAR(64) NOT NULL,
		library_name VARCHAR(255),
		library_version VARCHAR(100),
		identified_by VARCHAR(50),
		scanned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		date DATE,
		INDEX idx_library (library_name),
		INDEX idx_checksum (checksum)
	);`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// Create nmap_batches table for tracking port scan batches
	nmapQuery := `
	CREATE TABLE IF NOT EXISTS nmap_batches (
		id INT AUTO_INCREMENT PRIMARY KEY,
		batch_id VARCHAR(255) NOT NULL UNIQUE,
		url VARCHAR(2083) NOT NULL,
		status VARCHAR(50) NOT NULL,
		ports TEXT,
		results TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		INDEX idx_batch_id (batch_id),
		INDEX idx_status (status)
	);`
	_, err := db.Exec(nmapQuery)
	return err
}

// storeResult stores a scan result in the database.
func storeResult(result ScanResult) error {
	query := "INSERT INTO scan_results (url, script_url, checksum, library_name, library_version, identified_by, date) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := db.Exec(query, result.URL, result.ScriptURL, result.Checksum, result.LibraryName, result.LibraryVersion, result.IdentifiedBy, time.Now().Format("2006-01-02"))
	return err
}

// Statistics represents overall scan statistics
type Statistics struct {
	TotalURLs       int
	TotalScripts    int
	UniqueLibraries int
	FirstScan       *time.Time
	LastScan        *time.Time
}

// LibraryUsage represents library usage statistics
type LibraryUsage struct {
	Name         string
	Version      string
	Checksum     string
	Count        int
	IdentifiedBy string
}

// RecentScan represents a recent scan entry
type RecentScan struct {
	URL       string
	ScannedAt time.Time
}

// getOverallStatistics retrieves overall statistics from the database
func getOverallStatistics() (*Statistics, error) {
	stats := &Statistics{}
	
	// Get total unique URLs
	err := db.QueryRow("SELECT COUNT(DISTINCT url) FROM scan_results").Scan(&stats.TotalURLs)
	if err != nil {
		return nil, err
	}
	
	// Get total scripts
	err = db.QueryRow("SELECT COUNT(*) FROM scan_results").Scan(&stats.TotalScripts)
	if err != nil {
		return nil, err
	}
	
	// Get unique libraries (excluding Unknown and empty)
	err = db.QueryRow("SELECT COUNT(DISTINCT library_name) FROM scan_results WHERE library_name IS NOT NULL AND library_name != '' AND library_name != 'Unknown'").Scan(&stats.UniqueLibraries)
	if err != nil {
		return nil, err
	}
	
	// Get first and last scan times
	var firstScan, lastScan sql.NullTime
	err = db.QueryRow("SELECT MIN(scanned_at), MAX(scanned_at) FROM scan_results").Scan(&firstScan, &lastScan)
	if err != nil {
		return nil, err
	}
	
	if firstScan.Valid {
		stats.FirstScan = &firstScan.Time
	}
	if lastScan.Valid {
		stats.LastScan = &lastScan.Time
	}
	
	return stats, nil
}

// getLibraryStatistics retrieves library usage statistics
func getLibraryStatistics() ([]LibraryUsage, error) {
	query := `
		SELECT 
			library_name, 
			COALESCE(library_version, '') as library_version,
			checksum,
			COUNT(*) as count,
			MAX(identified_by) as identified_by
		FROM scan_results 
		WHERE library_name IS NOT NULL AND library_name != '' 
		GROUP BY library_name, library_version, checksum 
		ORDER BY count DESC, library_name ASC, library_version ASC
	`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var libraries []LibraryUsage
	for rows.Next() {
		var lib LibraryUsage
		if err := rows.Scan(&lib.Name, &lib.Version, &lib.Checksum, &lib.Count, &lib.IdentifiedBy); err != nil {
			return nil, err
		}
		libraries = append(libraries, lib)
	}
	
	return libraries, rows.Err()
}

// getRecentScans retrieves the most recent scans
func getRecentScans(limit int) ([]RecentScan, error) {
	query := `
		SELECT DISTINCT url, MAX(scanned_at) as last_scan 
		FROM scan_results 
		GROUP BY url 
		ORDER BY last_scan DESC 
		LIMIT ?
	`
	
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var scans []RecentScan
	for rows.Next() {
		var scan RecentScan
		if err := rows.Scan(&scan.URL, &scan.ScannedAt); err != nil {
			return nil, err
		}
		scans = append(scans, scan)
	}
	
	return scans, rows.Err()
}

// getNmapBatchStatistics retrieves nmap batch statistics
func getNmapBatchStatistics() (map[string]int, error) {
	query := `
		SELECT status, COUNT(*) as count 
		FROM nmap_batches 
		GROUP BY status
	`
	
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[status] = count
	}
	
	return stats, rows.Err()
}
