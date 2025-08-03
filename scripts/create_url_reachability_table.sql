-- Create url_reachability table for NetWeather application
-- This table tracks HTTP/HTTPS availability and redirect information for scanned URLs

USE netweather;

-- Create url_reachability table
CREATE TABLE IF NOT EXISTS url_reachability (
    id INT AUTO_INCREMENT PRIMARY KEY,
    original_url VARCHAR(2083) NOT NULL COMMENT 'The original URL from input file',
    http_available BOOLEAN DEFAULT FALSE COMMENT 'Whether URL is reachable via HTTP',
    https_available BOOLEAN DEFAULT FALSE COMMENT 'Whether URL is reachable via HTTPS',
    http_status_code INT COMMENT 'HTTP response status code',
    https_status_code INT COMMENT 'HTTPS response status code',
    http_redirect_url VARCHAR(2083) COMMENT 'URL after HTTP redirect (if any)',
    https_redirect_url VARCHAR(2083) COMMENT 'URL after HTTPS redirect (if any)',
    final_url VARCHAR(2083) COMMENT 'Final URL after all redirects',
    scanned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Timestamp when the reachability check was performed',
    INDEX idx_original_url (original_url),
    INDEX idx_scanned_at (scanned_at),
    INDEX idx_availability (http_available, https_available)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tracks HTTP/HTTPS reachability and redirect information for URLs';

-- Show table structure
DESCRIBE url_reachability;

-- Show confirmation
SELECT 'URL reachability table created successfully' AS Result;