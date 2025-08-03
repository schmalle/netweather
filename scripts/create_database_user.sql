-- Create database and user for NetWeather application
-- Run this script as a MySQL root or admin user

-- Create database
CREATE DATABASE IF NOT EXISTS netweather;

-- Create user with password
CREATE USER IF NOT EXISTS 'netweather'@'localhost' IDENTIFIED BY 'netweather';

-- Grant all privileges on netweather database to the user
GRANT ALL PRIVILEGES ON netweather.* TO 'netweather'@'localhost';

-- Apply privilege changes
FLUSH PRIVILEGES;

-- Show confirmation
SELECT 'Database and user created successfully' AS Result;