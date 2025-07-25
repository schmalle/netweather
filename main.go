package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/net/html"
)

func main() {
	initLogger("netweather.log")
	logger.Println("Application started")

	fmt.Println("NetWeather - URL Scanner")
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	filePath := os.Args[1]
	urls, err := readLines(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	if err := createTable(); err != nil {
		logger.Fatalf("Could not create table: %v", err)
	}

	for _, url := range urls {
		logger.Printf("Scanning URL: %s\n", url)
		fmt.Printf("Scanning URL: %s\n", url)
		scanURL(url)
	}
	logger.Println("Application finished")
}

func scanURL(baseURL string) {
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
	fmt.Println("Usage: netweather <url_file>")
	fmt.Println("Options:")
	fmt.Println("  <url_file>   File containing a list of URLs to scan.")
}

var logger *log.Logger

func initLogger(logFile string) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}

	logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)
}
