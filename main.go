package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/net/html"
)

func main() {
	// Define command line flags
	var (
		useDB       = flag.Bool("db", false, "Activate database storage")
		dbHost      = flag.String("db-host", "", "Database host")
		dbPort      = flag.String("db-port", "", "Database port")
		dbUser      = flag.String("db-user", "", "Database user")
		dbPassword  = flag.String("db-password", "", "Database password")
		dbName      = flag.String("db-name", "", "Database name")
		stats       = flag.Bool("stats", false, "Show statistics of scanned URLs")
		portScan    = flag.Bool("port-scan", false, "Enable port scanning with nmap")
		scanPorts   = flag.String("scan-ports", "80,443,8080,8443", "Ports to scan (default: common web ports)")
		nmapOptions = flag.String("nmap-options", "", "Additional nmap options")
	)
	flag.Parse()

	initLogger("netweather.log")
	logger.Println("Application started")

	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		logger.Println("No .env file found")
	}

	fmt.Println("NetWeather - URL Scanner")
	
	// Check if stats flag is set
	if *stats {
		// Stats mode requires database connection
		*useDB = true
	}

	// Initialize database if flag is set or stats is requested
	if *useDB || *stats {
		// Get database credentials from command line or environment variables
		host := getConfigValue(*dbHost, "DB_HOST", "127.0.0.1")
		port := getConfigValue(*dbPort, "DB_PORT", "3306")
		user := getConfigValue(*dbUser, "DB_USER", "")
		password := getConfigValue(*dbPassword, "DB_PASSWORD", "")
		database := getConfigValue(*dbName, "DB_NAME", "")

		if user == "" || database == "" {
			logger.Fatal("Database user and name must be provided via command line or environment variables")
		}

		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, database)
		if err := initDB(dsn); err != nil {
			logger.Fatalf("Could not initialize database: %v", err)
		}

		if err := createTable(); err != nil {
			logger.Fatalf("Could not create table: %v", err)
		}
	}

	// If stats flag is set, show statistics and exit
	if *stats {
		showStatistics()
		os.Exit(0)
	}

	// Regular scanning mode requires a URL file
	if flag.NArg() < 1 {
		printHelp()
		os.Exit(1)
	}

	filePath := flag.Arg(0)
	urls, err := readLines(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	for _, url := range urls {
		logger.Printf("Scanning URL: %s\n", url)
		fmt.Printf("Scanning URL: %s\n", url)
		scanURL(url, *useDB)
		
		// Perform port scan if enabled
		if *portScan {
			logger.Printf("Port scanning URL: %s\n", url)
			fmt.Printf("  - Port scanning: %s\n", url)
			performPortScan(url, *scanPorts, *nmapOptions)
		}
	}
	logger.Println("Application finished")
}

func scanURL(baseURL string, useDB bool) {
	logger.Printf("Fetching URL %s\n", baseURL)
	resp, err := http.Get(baseURL)
	if err != nil {
		logger.Printf("Error fetching URL %s: %v\n", baseURL, err)
		fmt.Printf("Error fetching URL %s: %v\n", baseURL, err)
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		logger.Printf("Error parsing HTML from %s: %v\n", baseURL, err)
		fmt.Printf("Error parsing HTML from %s: %v\n", baseURL, err)
		return
	}

	var scripts []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, a := range n.Attr {
				if a.Key == "src" {
					scripts = append(scripts, a.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	for _, scriptURL := range scripts {
		fullScriptURL := toAbsoluteURL(baseURL, scriptURL)
		logger.Printf("Processing script %s\n", fullScriptURL)
		checksum, err := getScriptChecksum(fullScriptURL)
		if err != nil {
			logger.Printf("Error processing script %s: %v\n", fullScriptURL, err)
			fmt.Printf("Error processing script %s: %v\n", fullScriptURL, err)
			continue
		}
		logger.Printf("Found script: %s, Checksum: %s\n", fullScriptURL, checksum)
		fmt.Printf("  - Found script: %s, Checksum: %s\n", fullScriptURL, checksum)

		libraryName, err := identifyLibrary(checksum)
		if err != nil {
			logger.Printf("Error identifying library for checksum %s: %v\n", checksum, err)
		} else {
			logger.Printf("Identified library for %s as: %s\n", fullScriptURL, libraryName)
			fmt.Printf("    Library: %s\n", libraryName)
		}

		if useDB {
			result := ScanResult{
				URL:         baseURL,
				ScriptURL:   fullScriptURL,
				Checksum:    checksum,
				LibraryName: libraryName,
			}
			if err := storeResult(result); err != nil {
				logger.Printf("Error storing result for %s: %v\n", fullScriptURL, err)
			}
		}
	}
}

func toAbsoluteURL(base, href string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	hrefURL, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(hrefURL).String()
}

func getScriptChecksum(scriptURL string) (string, error) {
	logger.Printf("Getting checksum for %s\n", scriptURL)
	resp, err := http.Get(scriptURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("Error reading script body from %s: %v\n", scriptURL, err)
		return "", err
	}

	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:]), nil
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func printHelp() {
	fmt.Println("Usage: netweather [options] <url_file>")
	fmt.Println("       netweather -stats [db-options]")
	fmt.Println("Options:")
	fmt.Println("  -db              Activate database storage")
	fmt.Println("  -db-host         Database host (default: 127.0.0.1, env: DB_HOST)")
	fmt.Println("  -db-port         Database port (default: 3306, env: DB_PORT)")
	fmt.Println("  -db-user         Database user (env: DB_USER)")
	fmt.Println("  -db-password     Database password (env: DB_PASSWORD)")
	fmt.Println("  -db-name         Database name (env: DB_NAME)")
	fmt.Println("  -stats           Show statistics of scanned URLs")
	fmt.Println("  -port-scan       Enable port scanning with nmap")
	fmt.Println("  -scan-ports      Ports to scan (default: 80,443,8080,8443)")
	fmt.Println("  -nmap-options    Additional nmap options")
	fmt.Println("  <url_file>       File containing a list of URLs to scan.")
}

// getConfigValue returns the first non-empty value from command line, environment, or default
func getConfigValue(cmdValue, envKey, defaultValue string) string {
	if cmdValue != "" {
		return cmdValue
	}
	if envValue := os.Getenv(envKey); envValue != "" {
		return envValue
	}
	return defaultValue
}

// showStatistics displays statistics from the database
func showStatistics() {
	fmt.Println("\n=== NetWeather Statistics ===\n")
	
	// Get overall statistics
	stats, err := getOverallStatistics()
	if err != nil {
		fmt.Printf("Error retrieving statistics: %v\n", err)
		return
	}
	
	fmt.Printf("Total URLs scanned: %d\n", stats.TotalURLs)
	fmt.Printf("Total scripts found: %d\n", stats.TotalScripts)
	fmt.Printf("Unique libraries identified: %d\n", stats.UniqueLibraries)
	
	if stats.FirstScan != nil {
		fmt.Printf("First scan: %s\n", stats.FirstScan.Format("2006-01-02 15:04:05"))
	}
	if stats.LastScan != nil {
		fmt.Printf("Last scan: %s\n", stats.LastScan.Format("2006-01-02 15:04:05"))
	}
	
	// Get library usage statistics
	fmt.Println("\n=== Library Usage ===")
	libraries, err := getLibraryStatistics()
	if err != nil {
		fmt.Printf("Error retrieving library statistics: %v\n", err)
		return
	}
	
	if len(libraries) == 0 {
		fmt.Println("No libraries found in database.")
		return
	}
	
	fmt.Println()
	for _, lib := range libraries {
		fmt.Printf("%-40s: %d occurrences\n", lib.Name, lib.Count)
	}
	
	// Get recent scans
	fmt.Println("\n=== Recent Scans ===")
	recentURLs, err := getRecentScans(10)
	if err != nil {
		fmt.Printf("Error retrieving recent scans: %v\n", err)
		return
	}
	
	if len(recentURLs) == 0 {
		fmt.Println("No recent scans found.")
		return
	}
	
	fmt.Println()
	for _, scan := range recentURLs {
		fmt.Printf("%s - %s\n", scan.ScannedAt.Format("2006-01-02 15:04:05"), scan.URL)
	}
	
	// Get nmap batch statistics
	fmt.Println("\n=== Port Scan Batches ===")
	nmapStats, err := getNmapBatchStatistics()
	if err != nil {
		fmt.Printf("Error retrieving batch statistics: %v\n", err)
		return
	}
	
	if len(nmapStats) == 0 {
		fmt.Println("No port scan batches found.")
		return
	}
	
	fmt.Println()
	for status, count := range nmapStats {
		fmt.Printf("%-15s: %d batches\n", status, count)
	}
}
