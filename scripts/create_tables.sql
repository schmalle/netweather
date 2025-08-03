-- Create tables for NetWeather application
-- This script creates all necessary database structures

USE netweather;

-- Drop existing table if needed (be careful with this in production!)
-- DROP TABLE IF EXISTS scan_results;

-- Create scan_results table
CREATE TABLE IF NOT EXISTS scan_results (
    id INT AUTO_INCREMENT PRIMARY KEY,
    url VARCHAR(2083) NOT NULL COMMENT 'The base URL that was scanned',
    script_url VARCHAR(2083) NOT NULL COMMENT 'The URL of the JavaScript file found',
    checksum VARCHAR(64) NOT NULL COMMENT 'SHA-256 checksum of the JavaScript file',
    library_name VARCHAR(255) COMMENT 'Identified library name from API',
    scanned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Timestamp when the scan was performed',
    date DATE COMMENT 'Date of the scan (for daily aggregation)',
    INDEX idx_url (url),
    INDEX idx_date (date),
    INDEX idx_checksum (checksum)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Stores results of website JavaScript library scans';

-- Show table structure
DESCRIBE scan_results;

-- Show confirmation
SELECT 'Tables created successfully' AS Result;