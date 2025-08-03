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
	"strings"

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
		useRemoteDB = flag.Bool("remote-db", false, "Use remote entries.db from GitHub instead of local file")
		verbose     = flag.Bool("verbose", false, "Enable verbose output (shows all URLs including non-200 responses)")
	)
	flag.Parse()

	initLogger("netweather.log")
	logger.Println("Application started")

	// Configure remote database if flag is set
	if *useRemoteDB {
		SetRemoteDB(true)
		logger.Println("Remote database mode enabled")
	}

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

	totalURLs := len(urls)
	processedCount := 0
	scannedCount := 0
	skippedCount := 0
	errorCount := 0
	
	// Show initial progress in non-verbose mode
	if !*verbose {
		fmt.Printf("Processing %d URLs...\n", totalURLs)
		fmt.Print("Progress: ")
	}
	
	for _, url := range urls {
		processedCount++
		logger.Printf("Processing URL: %s\n", url)
		
		if *verbose {
			fmt.Printf("\nProcessing URL: %s\n", url)
		}
		
		// First check URL reachability
		reachability, err := checkURLReachability(url)
		if err != nil {
			errorCount++
			logger.Printf("Error checking reachability for %s: %v\n", url, err)
			if *verbose {
				fmt.Printf("  - Error checking reachability: %v\n", err)
			}
			updateProgress(processedCount, totalURLs, *verbose)
			continue
		}
		
		// Display reachability information
		if reachability.HTTPAvailable || reachability.HTTPSAvailable {
			if *verbose {
				protocols := []string{}
				if reachability.HTTPAvailable {
					protocols = append(protocols, fmt.Sprintf("HTTP (%d)", reachability.HTTPStatusCode))
				}
				if reachability.HTTPSAvailable {
					protocols = append(protocols, fmt.Sprintf("HTTPS (%d)", reachability.HTTPSStatusCode))
				}
				fmt.Printf("  - Reachable via: %s\n", strings.Join(protocols, ", "))
				
				if reachability.HTTPRedirectURL != "" || reachability.HTTPSRedirectURL != "" {
					fmt.Printf("  - Redirects detected\n")
				}
				
				if reachability.FinalURL != "" && reachability.FinalURL != url {
					fmt.Printf("  - Final URL: %s\n", reachability.FinalURL)
				}
			}
		} else {
			errorCount++
			if *verbose {
				fmt.Printf("  - URL not reachable\n")
			}
			logger.Printf("URL %s is not reachable\n", url)
			
			// Store reachability result even if not reachable
			if *useDB {
				if err := storeURLReachability(reachability); err != nil {
					logger.Printf("Error storing reachability data for %s: %v\n", url, err)
				}
			}
			updateProgress(processedCount, totalURLs, *verbose)
			continue
		}
		
		// Store reachability data in database
		if *useDB {
			if err := storeURLReachability(reachability); err != nil {
				logger.Printf("Error storing reachability data for %s: %v\n", url, err)
			}
		}
		
		// Check if we got a successful response (HTTP 200)
		if !reachability.HasSuccessfulResponse() {
			skippedCount++
			logger.Printf("Skipping JavaScript scanning for %s - no HTTP 200 response (HTTP: %d, HTTPS: %d)\n", 
				url, reachability.HTTPStatusCode, reachability.HTTPSStatusCode)
			if *verbose {
				fmt.Printf("  - Skipping JavaScript scan (no HTTP 200 response)\n")
			}
			updateProgress(processedCount, totalURLs, *verbose)
			continue
		}
		
		// Scan the final URL (after redirects)
		finalURL := reachability.FinalURL
		if finalURL == "" {
			finalURL = url
		}
		
		scannedCount++
		logger.Printf("Scanning URL: %s\n", finalURL)
		
		if *verbose {
			fmt.Printf("  - Scanning for JavaScript libraries...\n")
		} else {
			// Show which URL we're scanning in non-verbose mode
			fmt.Printf("\n[%d/%d] Scanning: %s", processedCount, totalURLs, finalURL)
		}
		
		scanURL(finalURL, *useDB, *verbose)
		
		// Perform port scan if enabled
		if *portScan {
			logger.Printf("Port scanning URL: %s\n", finalURL)
			if *verbose {
				fmt.Printf("  - Port scanning: %s\n", finalURL)
			}
			performPortScan(finalURL, *scanPorts, *nmapOptions)
		}
		
		updateProgress(processedCount, totalURLs, *verbose)
	}
	
	// Final summary
	if !*verbose {
		fmt.Printf("\n\nScan completed!\n")
		fmt.Printf("Total URLs processed: %d\n", processedCount)
		fmt.Printf("Successfully scanned: %d\n", scannedCount)
		fmt.Printf("Skipped (non-200): %d\n", skippedCount)
		fmt.Printf("Errors/Unreachable: %d\n", errorCount)
	}
	logger.Println("Application finished")
}

// updateProgress shows progress indicator for non-verbose mode
func updateProgress(current, total int, verbose bool) {
	if !verbose {
		// Simple progress dots
		if current%10 == 0 || current == total {
			fmt.Printf(" %d", current)
		} else {
			fmt.Print(".")
		}
	}
}

func scanURL(baseURL string, useDB bool, verbose bool) {
	logger.Printf("Fetching URL %s\n", baseURL)
	resp, err := http.Get(baseURL)
	if err != nil {
		logger.Printf("Error fetching URL %s: %v\n", baseURL, err)
		if verbose {
			fmt.Printf("Error fetching URL %s: %v\n", baseURL, err)
		}
		return
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		logger.Printf("Error parsing HTML from %s: %v\n", baseURL, err)
		if verbose {
			fmt.Printf("Error parsing HTML from %s: %v\n", baseURL, err)
		}
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

	scriptsFound := 0
	for _, scriptURL := range scripts {
		fullScriptURL := toAbsoluteURL(baseURL, scriptURL)
		logger.Printf("Processing script %s\n", fullScriptURL)
		checksum, jsCode, err := getScriptChecksumAndContent(fullScriptURL)
		if err != nil {
			logger.Printf("Error processing script %s: %v\n", fullScriptURL, err)
			if verbose {
				fmt.Printf("Error processing script %s: %v\n", fullScriptURL, err)
			}
			continue
		}
		scriptsFound++
		logger.Printf("Found script: %s, Checksum: %s\n", fullScriptURL, checksum)
		
		if verbose {
			fmt.Printf("  - Found script: %s, Checksum: %s\n", fullScriptURL, checksum)
		}

		libraryInfo := identifyLibrary(fullScriptURL, checksum, jsCode)
		if libraryInfo != nil {
			logger.Printf("Identified library for %s as: %s v%s (%s) [checksum: %s]\n", fullScriptURL, libraryInfo.Name, libraryInfo.Version, libraryInfo.Method, libraryInfo.Checksum)
			if verbose {
				if libraryInfo.Version != "unknown" && libraryInfo.Version != "" {
					fmt.Printf("    Library: %s v%s (%s) [%s...]\n", libraryInfo.Name, libraryInfo.Version, libraryInfo.Method, libraryInfo.Checksum[:8])
				} else {
					fmt.Printf("    Library: %s (%s) [%s...]\n", libraryInfo.Name, libraryInfo.Method, libraryInfo.Checksum[:8])
				}
			}
		}

		if useDB && libraryInfo != nil {
			result := ScanResult{
				URL:              baseURL,
				ScriptURL:        fullScriptURL,
				Checksum:         checksum,
				LibraryName:      libraryInfo.Name,
				LibraryVersion:   libraryInfo.Version,
				IdentifiedBy:     libraryInfo.Method,
			}
			if err := storeResult(result); err != nil {
				logger.Printf("Error storing result for %s: %v\n", fullScriptURL, err)
			}
		}
	}
	
	// Show summary for non-verbose mode
	if !verbose {
		fmt.Printf(" → %d scripts found", scriptsFound)
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

func getScriptChecksumAndContent(scriptURL string) (string, string, error) {
	logger.Printf("Getting checksum and content for %s\n", scriptURL)
	resp, err := http.Get(scriptURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Printf("Error reading script body from %s: %v\n", scriptURL, err)
		return "", "", err
	}

	hash := sha256.Sum256(body)
	checksum := hex.EncodeToString(hash[:])
	content := string(body)
	
	return checksum, content, nil
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
	fmt.Println("  -remote-db       Use remote entries.db from GitHub")
	fmt.Println("  -verbose         Enable verbose output (default: false)")
	fmt.Println("  <url_file>       File containing a list of URLs to scan.")
	fmt.Println()
	fmt.Println("Features:")
	fmt.Println("  - Checks URL reachability via HTTP and HTTPS")
	fmt.Println("  - Follows redirects and scans the final URL")
	fmt.Println("  - Stores reachability information in database")
	fmt.Println("  - Handles URLs without protocol prefix (tests both HTTP/HTTPS)")
	fmt.Println("  - Skips JavaScript scanning for non-200 responses")
	fmt.Println("  - Clean, progress-based output in non-verbose mode")
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
		checksumDisplay := lib.Checksum
		if len(checksumDisplay) > 8 {
			checksumDisplay = checksumDisplay[:8] + "..."
		}
		
		if lib.Version != "" && lib.Version != "unknown" {
			fmt.Printf("%-25s v%-8s [%11s]: %d occurrences (%s)\n", lib.Name, lib.Version, checksumDisplay, lib.Count, lib.IdentifiedBy)
		} else {
			fmt.Printf("%-35s [%11s]: %d occurrences (%s)\n", lib.Name, checksumDisplay, lib.Count, lib.IdentifiedBy)
		}
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
	
	// Get URL reachability statistics
	fmt.Println("\n=== URL Reachability ===")
	reachStats, err := getURLReachabilityStatistics()
	if err != nil {
		fmt.Printf("Error retrieving reachability statistics: %v\n", err)
	} else if reachStats.TotalChecked > 0 {
		fmt.Println()
		fmt.Printf("Total URLs checked: %d\n", reachStats.TotalChecked)
		fmt.Printf("HTTP only: %d\n", reachStats.HTTPOnlyCount)
		fmt.Printf("HTTPS only: %d\n", reachStats.HTTPSOnlyCount)
		fmt.Printf("Both HTTP & HTTPS: %d\n", reachStats.BothProtocolsCount)
		fmt.Printf("Unreachable: %d\n", reachStats.UnreachableCount)
		fmt.Printf("URLs with redirects: %d\n", reachStats.RedirectCount)
	} else {
		fmt.Println("No URL reachability data found.")
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
