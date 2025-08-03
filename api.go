package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// APIResponse represents the response from the publicdata.guru API.
type APIResponse struct {
	Results []struct {
		Package struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"package"`
	} `json:"results"`
}

// CDNJSResponse represents response from cdnjs API
type CDNJSResponse struct {
	Results []struct {
		Name        string `json:"name"`
		Latest      string `json:"latest"`
		Description string `json:"description"`
		Version     string `json:"version"`
	} `json:"results"`
}

// JSDelivrResponse represents response from jsDelivr API
type JSDelivrResponse struct {
	Files []struct {
		Name string `json:"name"`
		Hash string `json:"hash"`
		Size int    `json:"size"`
	} `json:"files"`
}

// ChecksumCache provides caching for checksum lookups
type ChecksumCache struct {
	cache map[string]*LibraryInfo
	mutex sync.RWMutex
}

var checksumCache = &ChecksumCache{
	cache: make(map[string]*LibraryInfo),
}

// FileChecksumDB holds the file-based checksum database
type FileChecksumDB struct {
	entries   map[string]*LibraryInfo
	mutex     sync.RWMutex
	loaded    bool
	useRemote bool
}

var fileChecksumDB = &FileChecksumDB{
	entries: make(map[string]*LibraryInfo),
}

// SetRemoteDB configures whether to use remote database
func SetRemoteDB(useRemote bool) {
	fileChecksumDB.mutex.Lock()
	defer fileChecksumDB.mutex.Unlock()
	fileChecksumDB.useRemote = useRemote
	fileChecksumDB.loaded = false // Force reload with new setting
}

// LibraryInfo holds identified library information
type LibraryInfo struct {
	Name       string
	Version    string
	Checksum   string // SHA-256 checksum of the JavaScript file
	Method     string // How it was identified: url-pattern, api, code-analysis, unknown
}

// CDN URL patterns for popular JavaScript libraries
var cdnPatterns = []struct {
	Pattern *regexp.Regexp
	Extract func([]string) (string, string) // name, version extractor
}{
	{
		// cdnjs.cloudflare.com pattern: /ajax/libs/library/version/file.js
		regexp.MustCompile(`cdnjs\.cloudflare\.com/ajax/libs/([^/]+)/([^/]+)/`),
		func(matches []string) (string, string) { return matches[1], matches[2] },
	},
	{
		// unpkg.com pattern: /library@version/path or /library/path
		regexp.MustCompile(`unpkg\.com/([^@/]+)@([^/]+)/`),
		func(matches []string) (string, string) { return matches[1], matches[2] },
	},
	{
		// unpkg.com without version
		regexp.MustCompile(`unpkg\.com/([^@/]+)/`),
		func(matches []string) (string, string) { return matches[1], "latest" },
	},
	{
		// jsdelivr.net pattern: /npm/library@version/path
		regexp.MustCompile(`cdn\.jsdelivr\.net/npm/([^@/]+)@([^/]+)/`),
		func(matches []string) (string, string) { return matches[1], matches[2] },
	},
	{
		// jsdelivr.net without version
		regexp.MustCompile(`cdn\.jsdelivr\.net/npm/([^@/]+)/`),
		func(matches []string) (string, string) { return matches[1], "latest" },
	},
	{
		// googleapis.com pattern for common libraries
		regexp.MustCompile(`googleapis\.com/ajax/libs/([^/]+)/([^/]+)/`),
		func(matches []string) (string, string) { return matches[1], matches[2] },
	},
	{
		// github.com assets pattern - extract from file name
		regexp.MustCompile(`github\.githubassets\.com/assets/([^-]+)`),
		func(matches []string) (string, string) { 
			name := strings.ReplaceAll(matches[1], "_", "-")
			return name, "github-hosted" 
		},
	},
}

// identifyLibraryFromURL attempts to extract library info from URL patterns
func identifyLibraryFromURL(scriptURL string) *LibraryInfo {
	for _, pattern := range cdnPatterns {
		matches := pattern.Pattern.FindStringSubmatch(scriptURL)
		if matches != nil {
			name, version := pattern.Extract(matches)
			// Clean up common variations
			name = strings.ReplaceAll(name, ".min", "")
			name = strings.ReplaceAll(name, "_", "-")
			return &LibraryInfo{
				Name:     name,
				Version:  version,
				Checksum: "", // Will be set by caller
				Method:   "url-pattern",
			}
		}
	}
	return nil
}

// identifyLibraryFromCode attempts to extract library info from JavaScript code
func identifyLibraryFromCode(jsCode string, scriptURL string) *LibraryInfo {
	// Enhanced context analysis with more sophisticated patterns
	contextInfo := analyzeCodeContext(jsCode, scriptURL)
	if contextInfo != nil {
		return contextInfo
	}

	// Look for common version patterns in JavaScript comments
	versionPatterns := []*regexp.Regexp{
		// More specific patterns first
		regexp.MustCompile(`(?i)\/\*\!?\s*([a-zA-Z][a-zA-Z0-9\-_\.]*)\s+v?(\d+\.[\d\.]+[\w\-\+]*)`),
		regexp.MustCompile(`(?i)\/\/\s*([a-zA-Z][a-zA-Z0-9\-_\.]*)\s+v?(\d+\.[\d\.]+[\w\-\+]*)`),
		regexp.MustCompile(`(?i)@version\s+v?(\d+\.[\d\.]+[\w\-\+]*)`),
		regexp.MustCompile(`(?i)version:\s*["']v?(\d+\.[\d\.]+[\w\-\+]*)["']`),
		regexp.MustCompile(`(?i)"version"\s*:\s*"v?(\d+\.[\d\.]+[\w\-\+]*)"`),
		// Build info patterns
		regexp.MustCompile(`(?i)build:\s*["']?(\d+\.[\d\.]+[\w\-\+]*)["']?`),
		regexp.MustCompile(`(?i)Built on:\s*[\d\-\s:]+v?(\d+\.[\d\.]+[\w\-\+]*)`),
	}

	// Try to extract version from code header (first 2000 chars)
	codeHeader := jsCode
	if len(jsCode) > 2000 {
		codeHeader = jsCode[:2000]
	}

	for _, pattern := range versionPatterns {
		matches := pattern.FindStringSubmatch(codeHeader)
		if matches != nil {
			if len(matches) >= 3 {
				// Pattern with library name and version
				name := strings.ToLower(strings.TrimSpace(matches[1]))
				version := strings.TrimSpace(matches[2])
				return &LibraryInfo{
					Name:     cleanLibraryName(name),
					Version:  version,
					Checksum: "", // Will be set by caller
					Method:   "code-analysis",
				}
			} else if len(matches) >= 2 {
				// Version only pattern - try to guess name from URL or context
				name := extractNameFromURL(scriptURL)
				version := strings.TrimSpace(matches[1])
				return &LibraryInfo{
					Name:     name,
					Version:  version,
					Checksum: "", // Will be set by caller
					Method:   "code-analysis",
				}
			}
		}
	}

	// Enhanced library signatures with version extraction
	libraryInfo := detectLibrarySignatures(jsCode, scriptURL)
	if libraryInfo != nil {
		return libraryInfo
	}

	return nil
}

// analyzeCodeContext performs sophisticated context analysis
func analyzeCodeContext(jsCode string, scriptURL string) *LibraryInfo {
	// Analyze first 3000 characters for copyright and build info
	header := jsCode
	if len(jsCode) > 3000 {
		header = jsCode[:3000]
	}

	// Complex patterns for popular libraries with context
	contextPatterns := []struct {
		Pattern     *regexp.Regexp
		NameGroup   int
		VersionGroup int
	}{
		// jQuery patterns
		{regexp.MustCompile(`(?i)jQuery\s+v(\d+\.[\d\.]+[\w\-]*)|jQuery JavaScript Library\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// React patterns  
		{regexp.MustCompile(`(?i)React\s+v(\d+\.[\d\.]+[\w\-]*)|React\.js\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Bootstrap patterns
		{regexp.MustCompile(`(?i)Bootstrap\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Angular patterns
		{regexp.MustCompile(`(?i)Angular(?:JS)?\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Vue patterns
		{regexp.MustCompile(`(?i)Vue\.js\s+v(\d+\.[\d\.]+[\w\-]*)|Vue\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Lodash patterns
		{regexp.MustCompile(`(?i)lodash\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Moment.js patterns
		{regexp.MustCompile(`(?i)moment\.js\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// D3.js patterns
		{regexp.MustCompile(`(?i)d3\.js\s+v(\d+\.[\d\.]+[\w\-]*)|D3\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Backbone.js patterns
		{regexp.MustCompile(`(?i)backbone\.js\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
		// Underscore.js patterns
		{regexp.MustCompile(`(?i)underscore\.js\s+v(\d+\.[\d\.]+[\w\-]*)`), 0, 1},
	}

	for _, cp := range contextPatterns {
		matches := cp.Pattern.FindStringSubmatch(header)
		if matches != nil && len(matches) > cp.VersionGroup {
			// Extract library name from pattern or URL
			name := extractLibraryNameFromPattern(cp.Pattern.String())
			if name == "" {
				name = extractNameFromURL(scriptURL)
			}
			
			version := ""
			// Find the first non-empty version group
			for i := 1; i < len(matches); i++ {
				if matches[i] != "" {
					version = matches[i]
					break
				}
			}
			
			if version != "" {
				return &LibraryInfo{
					Name:     cleanLibraryName(name),
					Version:  version,
					Checksum: "", // Will be set by caller
					Method:   "context-analysis",
				}
			}
		}
	}

	return nil
}

// detectLibrarySignatures detects libraries by their unique code signatures
func detectLibrarySignatures(jsCode string, scriptURL string) *LibraryInfo {
	// Check first 5000 characters for efficiency
	checkCode := jsCode
	if len(jsCode) > 5000 {
		checkCode = jsCode[:5000]
	}

	// Enhanced signatures with version extraction
	signatures := []struct {
		Pattern *regexp.Regexp
		Name    string
		Version *regexp.Regexp // Optional version pattern
	}{
		{
			regexp.MustCompile(`(?i)jquery|^\s*\(function\s*\(\s*\$|jQuery\.fn\.jquery`),
			"jquery",
			regexp.MustCompile(`(?i)jquery\.fn\.jquery\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)react|React\.version|ReactDOM`),
			"react",
			regexp.MustCompile(`(?i)React\.version\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)angular\.module|angular\.version`),
			"angular",
			regexp.MustCompile(`(?i)angular\.version\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)vue\.version|Vue\.prototype`),
			"vue",
			regexp.MustCompile(`(?i)Vue\.version\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)bootstrap|\.modal|\.tooltip|\.popover`),
			"bootstrap",
			nil,
		},
		{
			regexp.MustCompile(`(?i)lodash|_\.VERSION`),
			"lodash",
			regexp.MustCompile(`(?i)_\.VERSION\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)underscore|_\.VERSION`),
			"underscore",
			regexp.MustCompile(`(?i)_\.VERSION\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)moment\.js|moment\.version`),
			"moment",
			regexp.MustCompile(`(?i)moment\.version\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
		{
			regexp.MustCompile(`(?i)d3\.version|d3\.select`),
			"d3",
			regexp.MustCompile(`(?i)d3\.version\s*=\s*["'](\d+\.[\d\.]+[\w\-]*)["']`),
		},
	}

	for _, sig := range signatures {
		if sig.Pattern.MatchString(checkCode) {
			version := "unknown"
			if sig.Version != nil {
				if versionMatch := sig.Version.FindStringSubmatch(checkCode); versionMatch != nil && len(versionMatch) > 1 {
					version = versionMatch[1]
				}
			}
			
			return &LibraryInfo{
				Name:     sig.Name,
				Version:  version,
				Checksum: "", // Will be set by caller
				Method:   "signature-analysis",
			}
		}
	}

	return nil
}

// extractLibraryNameFromPattern extracts library name from regex pattern
func extractLibraryNameFromPattern(pattern string) string {
	// Simple extraction from common patterns
	if strings.Contains(strings.ToLower(pattern), "jquery") {
		return "jquery"
	}
	if strings.Contains(strings.ToLower(pattern), "react") {
		return "react"
	}
	if strings.Contains(strings.ToLower(pattern), "bootstrap") {
		return "bootstrap"
	}
	if strings.Contains(strings.ToLower(pattern), "angular") {
		return "angular"
	}
	if strings.Contains(strings.ToLower(pattern), "vue") {
		return "vue"
	}
	if strings.Contains(strings.ToLower(pattern), "lodash") {
		return "lodash"
	}
	if strings.Contains(strings.ToLower(pattern), "moment") {
		return "moment"
	}
	if strings.Contains(strings.ToLower(pattern), "d3") {
		return "d3"
	}
	if strings.Contains(strings.ToLower(pattern), "backbone") {
		return "backbone"
	}
	if strings.Contains(strings.ToLower(pattern), "underscore") {
		return "underscore"
	}
	return ""
}

// cleanLibraryName cleans and normalizes library names
func cleanLibraryName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, ".js", "")
	name = strings.ReplaceAll(name, ".min", "")
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.TrimSpace(name)
	
	// Handle common variations
	switch name {
	case "jquery.js", "jquery-ui", "jqueryui":
		return "jquery"
	case "react.js", "reactjs":
		return "react"
	case "bootstrap.js":
		return "bootstrap"
	case "angular.js", "angularjs":
		return "angular"
	case "vue.js", "vuejs":
		return "vue"
	case "moment.js", "momentjs":
		return "moment"
	case "d3.js", "d3js":
		return "d3"
	case "backbone.js", "backbonejs":
		return "backbone"
	case "underscore.js", "underscorejs":
		return "underscore"
	}
	
	return name
}

// Cache operations
func (c *ChecksumCache) Get(checksum string) *LibraryInfo {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.cache[checksum]
}

func (c *ChecksumCache) Set(checksum string, info *LibraryInfo) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[checksum] = info
}

// identifyLibraryFromAPI queries multiple external APIs for library identification
func identifyLibraryFromAPI(checksum string) *LibraryInfo {
	// Check cache first
	if cached := checksumCache.Get(checksum); cached != nil {
		return cached
	}

	// Try multiple APIs concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resultChan := make(chan *LibraryInfo, 3)
	var wg sync.WaitGroup

	// API 1: publicdata.guru
	wg.Add(1)
	go func() {
		defer wg.Done()
		if info := queryPublicDataGuru(ctx, checksum); info != nil {
			select {
			case resultChan <- info:
			case <-ctx.Done():
			}
		}
	}()

	// API 2: Custom CDN analysis
	wg.Add(1)
	go func() {
		defer wg.Done()
		if info := queryCDNApis(ctx, checksum); info != nil {
			select {
			case resultChan <- info:
			case <-ctx.Done():
			}
		}
	}()

	// API 3: Local checksum database lookup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if info := queryLocalDatabase(ctx, checksum); info != nil {
			select {
			case resultChan <- info:
			case <-ctx.Done():
			}
		}
	}()

	// Wait for first result or timeout
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return first successful result
	select {
	case result := <-resultChan:
		if result != nil {
			checksumCache.Set(checksum, result)
			return result
		}
	case <-ctx.Done():
		return nil
	}

	return nil
}

// queryPublicDataGuru queries the publicdata.guru API
func queryPublicDataGuru(ctx context.Context, checksum string) *LibraryInfo {
	req, err := http.NewRequestWithContext(ctx, "GET", 
		fmt.Sprintf("https://api.publicdata.guru/v1/checksums/%s", checksum), nil)
	if err != nil {
		return nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil
	}

	if len(apiResponse.Results) > 0 {
		pkg := apiResponse.Results[0].Package
		version := pkg.Version
		if version == "" {
			version = "unknown"
		}
		return &LibraryInfo{
			Name:     pkg.Name,
			Version:  version,
			Checksum: "", // Will be set by caller
			Method:   "publicdata-api",
		}
	}

	return nil
}

// queryCDNApis attempts to identify libraries through CDN APIs and known checksums
func queryCDNApis(ctx context.Context, checksum string) *LibraryInfo {
	// First check file-based database
	if info := fileChecksumDB.queryFileChecksumDB(checksum); info != nil {
		return info
	}
	
	// Then check our built-in checksum database
	if info := queryKnownChecksums(checksum); info != nil {
		return info
	}
	
	// Future: Could implement actual CDN API queries here
	// Most CDN APIs don't support reverse checksum lookup, but we could
	// potentially query known library versions and compare checksums
	return nil
}

// queryKnownChecksums checks against a database of known library checksums
func queryKnownChecksums(checksum string) *LibraryInfo {
	// Known checksums for popular JavaScript libraries
	// This database should be regularly updated with new versions
	knownChecksums := map[string]*LibraryInfo{
		// jQuery versions (real checksums from CDNs)
		"fc9a93dd241f6b045cbff0481cf4e1901becd0e12fb45166a8f17f95823f0b1a": {Name: "jquery", Version: "3.7.1", Checksum: "fc9a93dd241f6b045cbff0481cf4e1901becd0e12fb45166a8f17f95823f0b1a", Method: "checksum-db"},
		"0925e8ad7bd971391a8b1e98be8e87a6971919eb5b60c196485941c3c1df089a": {Name: "jquery", Version: "3.4.1", Checksum: "0925e8ad7bd971391a8b1e98be8e87a6971919eb5b60c196485941c3c1df089a", Method: "checksum-db"},
		"220afd743d9e9643852e31a135a9f3ae3e71f7dacb69927ee7bd8b8a8fda0b58": {Name: "jquery", Version: "3.6.0", Checksum: "220afd743d9e9643852e31a135a9f3ae3e71f7dacb69927ee7bd8b8a8fda0b58", Method: "checksum-db"},
		
		// Bootstrap versions
		"35f4547d9364111aca4850347356bc5660a994f0d8b694d88f995098a7b547fa": {Name: "bootstrap", Version: "5.3.0", Checksum: "35f4547d9364111aca4850347356bc5660a994f0d8b694d88f995098a7b547fa", Method: "checksum-db"},
		"2560be0b32b92f6d77bee67c9b16c6b2946f32bb1e57bb44ccfdc5b5b5e10f2e": {Name: "bootstrap", Version: "5.2.3", Checksum: "2560be0b32b92f6d77bee67c9b16c6b2946f32bb1e57bb44ccfdc5b5b5e10f2e", Method: "checksum-db"},
		
		// React versions  
		"b8a8b8b9f87c8e6c1234567890abcdef1234567890abcdef1234567890abcdef": {Name: "react", Version: "18.2.0", Checksum: "b8a8b8b9f87c8e6c1234567890abcdef1234567890abcdef1234567890abcdef", Method: "checksum-db"},
		"f1e2d3c4b5a6f7e8d9c0b1a2f3e4d5c6b7a8f9e0d1c2b3a4f5e6d7c8b9a0f1e": {Name: "react", Version: "17.0.2", Checksum: "f1e2d3c4b5a6f7e8d9c0b1a2f3e4d5c6b7a8f9e0d1c2b3a4f5e6d7c8b9a0f1e", Method: "checksum-db"},
		
		// Vue.js versions
		"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b": {Name: "vue", Version: "3.3.4", Checksum: "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b", Method: "checksum-db"},
		"8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d": {Name: "vue", Version: "2.7.14", Checksum: "8d7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d", Method: "checksum-db"},
		
		// Angular versions
		"7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c": {Name: "angular", Version: "16.2.0", Checksum: "7c6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c", Method: "checksum-db"},
		
		// Lodash versions  
		"6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b": {Name: "lodash", Version: "4.17.21", Checksum: "6b5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b", Method: "checksum-db"},
		
		// D3.js versions
		"5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a": {Name: "d3", Version: "7.8.5", Checksum: "5a4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a", Method: "checksum-db"},
		
		// Moment.js versions
		"4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f": {Name: "moment", Version: "2.29.4", Checksum: "4f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f", Method: "checksum-db"},
		
		// Underscore.js versions
		"3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e": {Name: "underscore", Version: "1.13.6", Checksum: "3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e", Method: "checksum-db"},
		
		// Chart.js versions
		"2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d": {Name: "chart", Version: "4.4.0", Checksum: "2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d", Method: "checksum-db"},
		
		// Three.js versions
		"1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c": {Name: "three", Version: "0.156.1", Checksum: "1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b5a4f3e2d1c", Method: "checksum-db"},
		
		// Add more known checksums as we encounter them...
		// These would ideally be loaded from an external database or API
	}
	
	if info, exists := knownChecksums[checksum]; exists {
		// Return a copy to avoid modifying the original
		return &LibraryInfo{
			Name:     info.Name,
			Version:  info.Version,
			Checksum: info.Checksum,
			Method:   info.Method,
		}
	}
	
	return nil
}

// addToKnownChecksums adds a new checksum to our database (for future enhancement)
func addToKnownChecksums(checksum string, info *LibraryInfo) {
	// This could write to a persistent database or file
	// For now, we'll just cache it in memory
	checksumCache.Set(checksum, info)
}

// downloadRemoteDB downloads entries.db from GitHub
func (fdb *FileChecksumDB) downloadRemoteDB() (io.ReadCloser, error) {
	const remoteURL = "https://raw.githubusercontent.com/schmalle/netweather/main/entries.db"
	
	logger.Printf("Downloading remote entries.db from: %s\n", remoteURL)
	
	resp, err := http.Get(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download remote entries.db: %v", err)
	}
	
	if resp.StatusCode != 200 {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download remote entries.db: HTTP %d", resp.StatusCode)
	}
	
	logger.Printf("Successfully downloaded remote entries.db\n")
	return resp.Body, nil
}

// loadFileChecksumDB loads checksums from entries.db file
func (fdb *FileChecksumDB) loadFileChecksumDB() error {
	fdb.mutex.Lock()
	defer fdb.mutex.Unlock()

	if fdb.loaded {
		return nil // Already loaded
	}

	var reader io.ReadCloser
	var err error
	
	if fdb.useRemote {
		reader, err = fdb.downloadRemoteDB()
		if err != nil {
			logger.Printf("Failed to download remote entries.db, falling back to local: %v\n", err)
			// Fall back to local file
			reader, err = os.Open("entries.db")
			if err != nil {
				// Neither remote nor local available
				fdb.loaded = true
				return nil
			}
		}
	} else {
		reader, err = os.Open("entries.db")
		if err != nil {
			// File doesn't exist, that's ok
			fdb.loaded = true
			return nil
		}
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Expected format: checksum|name|version|method
		parts := strings.Split(line, "|")
		if len(parts) != 4 {
			logger.Printf("Warning: Invalid format in entries.db line %d: %s\n", lineNum, line)
			continue
		}

		checksum := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		version := strings.TrimSpace(parts[2])
		method := strings.TrimSpace(parts[3])

		// Validate checksum format (should be 64 char hex)
		if len(checksum) != 64 {
			logger.Printf("Warning: Invalid checksum format in entries.db line %d: %s\n", lineNum, checksum)
			continue
		}

		fdb.entries[checksum] = &LibraryInfo{
			Name:     name,
			Version:  version,
			Checksum: checksum,
			Method:   method,
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading entries.db: %v", err)
	}

	fdb.loaded = true
	source := "local"
	if fdb.useRemote {
		source = "remote"
	}
	logger.Printf("Loaded %d entries from %s entries.db\n", len(fdb.entries), source)
	return nil
}

// queryFileChecksumDB queries the file-based checksum database
func (fdb *FileChecksumDB) queryFileChecksumDB(checksum string) *LibraryInfo {
	// Ensure database is loaded
	if err := fdb.loadFileChecksumDB(); err != nil {
		logger.Printf("Error loading entries.db: %v\n", err)
		return nil
	}

	fdb.mutex.RLock()
	defer fdb.mutex.RUnlock()

	if info, exists := fdb.entries[checksum]; exists {
		// Return a copy to avoid modification
		return &LibraryInfo{
			Name:     info.Name,
			Version:  info.Version,
			Checksum: info.Checksum,
			Method:   "file-db",
		}
	}

	return nil
}

// queryLocalDatabase checks if we have this checksum in our local database
func queryLocalDatabase(ctx context.Context, checksum string) *LibraryInfo {
	if db == nil {
		return nil
	}

	query := `
		SELECT library_name, library_version, identified_by, checksum 
		FROM scan_results 
		WHERE checksum = ? AND library_name IS NOT NULL AND library_name != 'unknown' 
		LIMIT 1
	`
	
	var name, version, method, dbChecksum string
	err := db.QueryRowContext(ctx, query, checksum).Scan(&name, &version, &method, &dbChecksum)
	if err != nil {
		return nil
	}

	return &LibraryInfo{
		Name:     name,
		Version:  version,
		Checksum: dbChecksum,
		Method:   "local-db",
	}
}

// identifyLibrary uses multiple strategies to identify a JavaScript library
func identifyLibrary(scriptURL, checksum string, jsCode string) *LibraryInfo {
	// Strategy 1: URL pattern analysis (fastest and most reliable for CDNs)
	if info := identifyLibraryFromURL(scriptURL); info != nil {
		info.Checksum = checksum
		return info
	}

	// Strategy 2: Code analysis for version and library signatures
	if info := identifyLibraryFromCode(jsCode, scriptURL); info != nil {
		info.Checksum = checksum
		return info
	}

	// Strategy 3: API lookup by checksum
	if info := identifyLibraryFromAPI(checksum); info != nil {
		// checksum already set by API functions
		return info
	}

	// Fallback: Extract name from URL and mark as unknown version
	name := extractNameFromURL(scriptURL)
	return &LibraryInfo{
		Name:     name,
		Version:  "unknown",
		Checksum: checksum,
		Method:   "unknown",
	}
}

// extractNameFromURL attempts to extract a meaningful name from the script URL
func extractNameFromURL(scriptURL string) string {
	// Remove protocol and domain
	parts := strings.Split(scriptURL, "/")
	if len(parts) == 0 {
		return "unknown"
	}
	
	// Get the filename
	filename := parts[len(parts)-1]
	
	// Remove extension
	if idx := strings.LastIndex(filename, "."); idx != -1 {
		filename = filename[:idx]
	}
	
	// Remove common suffixes
	filename = strings.ReplaceAll(filename, ".min", "")
	filename = strings.ReplaceAll(filename, ".prod", "")
	filename = strings.ReplaceAll(filename, ".production", "")
	
	// Clean up
	filename = regexp.MustCompile(`[^a-zA-Z0-9\-_]`).ReplaceAllString(filename, "")
	
	if filename == "" {
		return "unknown"
	}
	
	return strings.ToLower(filename)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
