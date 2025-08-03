-- Delete all entries from NetWeather database tables
-- WARNING: This will permanently delete all scan results!

USE netweather;

-- Delete all entries from scan_results table
DELETE FROM scan_results;

-- Show number of rows remaining (should be 0)
SELECT COUNT(*) AS remaining_rows FROM scan_results;

-- Reset auto-increment counter
ALTER TABLE scan_results AUTO_INCREMENT = 1;

-- Show confirmation
SELECT 'All entries deleted successfully' AS Result;