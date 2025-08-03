package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	
	"golang.org/x/net/html"
)

// ParallelConfig holds configuration for parallel processing
type ParallelConfig struct {
	MaxWorkers   int
	RequestDelay time.Duration
	BatchSize    int
	UseDB        bool
	Verbose      bool
}

// URLJob represents a URL to be processed
type URLJob struct {
	URL           string
	Index         int
	OriginalIndex int
}

// URLResult represents the outcome of processing a URL
type URLResult struct {
	Job          URLJob
	Reachability *URLReachability
	ScanResults  []ScanResult
	Error        error
	Excluded     bool
	Skipped      bool
	ProcessTime  time.Duration
}

// ProgressTracker provides thread-safe progress tracking
type ProgressTracker struct {
	total     int64
	processed int64
	scanned   int64
	excluded  int64
	skipped   int64
	errors    int64
	verbose   bool
	mu        sync.RWMutex
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(total int, verbose bool) *ProgressTracker {
	return &ProgressTracker{
		total:   int64(total),
		verbose: verbose,
	}
}

// IncrementProcessed atomically increments processed counter
func (pt *ProgressTracker) IncrementProcessed() {
	atomic.AddInt64(&pt.processed, 1)
}

// IncrementScanned atomically increments scanned counter
func (pt *ProgressTracker) IncrementScanned() {
	atomic.AddInt64(&pt.scanned, 1)
}

// IncrementExcluded atomically increments excluded counter
func (pt *ProgressTracker) IncrementExcluded() {
	atomic.AddInt64(&pt.excluded, 1)
}

// IncrementSkipped atomically increments skipped counter
func (pt *ProgressTracker) IncrementSkipped() {
	atomic.AddInt64(&pt.skipped, 1)
}

// IncrementErrors atomically increments errors counter
func (pt *ProgressTracker) IncrementErrors() {
	atomic.AddInt64(&pt.errors, 1)
}

// GetCounts returns current counts atomically
func (pt *ProgressTracker) GetCounts() (processed, scanned, excluded, skipped, errors int64) {
	return atomic.LoadInt64(&pt.processed),
		atomic.LoadInt64(&pt.scanned),
		atomic.LoadInt64(&pt.excluded),
		atomic.LoadInt64(&pt.skipped),
		atomic.LoadInt64(&pt.errors)
}

// ParallelProcessor handles parallel URL processing
type ParallelProcessor struct {
	config  ParallelConfig
	tracker *ProgressTracker
	mu      sync.Mutex // For synchronized output
}

// NewParallelProcessor creates a new parallel processor
func NewParallelProcessor(config ParallelConfig) *ParallelProcessor {
	return &ParallelProcessor{
		config: config,
	}
}

// ProcessURLs processes URLs in parallel using worker pool pattern
func (pp *ParallelProcessor) ProcessURLs(ctx context.Context, urls []string) error {
	pp.tracker = NewProgressTracker(len(urls), pp.config.Verbose)
	
	// Validate worker count
	maxWorkers := pp.config.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 1
	}
	if maxWorkers > len(urls) {
		maxWorkers = len(urls)
	}
	
	// Create channels
	jobs := make(chan URLJob, len(urls))
	results := make(chan URLResult, maxWorkers*2) // Buffer for worker results
	
	// Start progress display (non-verbose mode)
	if !pp.config.Verbose {
		logger.Printf("Starting parallel processing with %d workers\n", maxWorkers)
		pp.mu.Lock()
		fmt.Printf("Processing %d URLs with %d workers...\n", len(urls), maxWorkers)
		fmt.Print("Progress: ")
		pp.mu.Unlock()
	}
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go pp.urlWorker(ctx, jobs, results, &wg)
	}
	
	// Start result collector
	collectorDone := make(chan struct{})
	go pp.resultCollector(results, len(urls), collectorDone)
	
	// Send jobs
	for i, url := range urls {
		select {
		case jobs <- URLJob{URL: url, Index: i, OriginalIndex: i}:
		case <-ctx.Done():
			close(jobs)
			return ctx.Err()
		}
	}
	close(jobs)
	
	// Wait for workers to complete
	wg.Wait()
	close(results)
	
	// Wait for result collector to finish
	<-collectorDone
	
	// Final summary
	pp.displayFinalSummary()
	
	return nil
}

// urlWorker processes URLs from the job queue
func (pp *ParallelProcessor) urlWorker(ctx context.Context, jobs <-chan URLJob, results chan<- URLResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		startTime := time.Now()
		result := pp.processURL(ctx, job)
		result.ProcessTime = time.Since(startTime)
		
		select {
		case results <- result:
		case <-ctx.Done():
			return
		}
		
		// Rate limiting
		if pp.config.RequestDelay > 0 {
			time.Sleep(pp.config.RequestDelay)
		}
	}
}

// processURL processes a single URL (core logic)
func (pp *ParallelProcessor) processURL(ctx context.Context, job URLJob) URLResult {
	result := URLResult{Job: job}
	
	pp.tracker.IncrementProcessed()
	logger.Printf("Processing URL: %s\n", job.URL)
	
	// Check if URL should be excluded
	if shouldExcludeURL(job.URL) {
		pp.tracker.IncrementExcluded()
		result.Excluded = true
		logger.Printf("Skipping excluded URL: %s\n", job.URL)
		return result
	}
	
	// Check URL reachability
	reachability, err := checkURLReachability(job.URL)
	if err != nil {
		pp.tracker.IncrementErrors()
		result.Error = err
		logger.Printf("Error checking reachability for %s: %v\n", job.URL, err)
		return result
	}
	
	result.Reachability = reachability
	
	// Store reachability data in database
	if pp.config.UseDB && reachability != nil {
		if err := storeURLReachability(reachability); err != nil {
			logger.Printf("Error storing reachability data for %s: %v\n", job.URL, err)
		}
	}
	
	// Check if URL is reachable
	if !reachability.HTTPAvailable && !reachability.HTTPSAvailable {
		pp.tracker.IncrementErrors()
		logger.Printf("URL %s is not reachable\n", job.URL)
		return result
	}
	
	// Check if we got a successful response (HTTP 200)
	if !reachability.HasSuccessfulResponse() {
		pp.tracker.IncrementSkipped()
		result.Skipped = true
		logger.Printf("Skipping JavaScript scanning for %s - no HTTP 200 response (HTTP: %d, HTTPS: %d)\n", 
			job.URL, reachability.HTTPStatusCode, reachability.HTTPSStatusCode)
		return result
	}
	
	// Scan the final URL (after redirects)
	finalURL := reachability.FinalURL
	if finalURL == "" {
		finalURL = job.URL
	}
	
	pp.tracker.IncrementScanned()
	logger.Printf("Scanning URL: %s\n", finalURL)
	
	// Perform JavaScript scanning
	scanResults := pp.scanURLForResults(finalURL)
	result.ScanResults = scanResults
	
	return result
}

// scanURLForResults performs JavaScript scanning and returns results
func (pp *ParallelProcessor) scanURLForResults(baseURL string) []ScanResult {
	var results []ScanResult
	
	logger.Printf("Fetching URL %s\n", baseURL)
	resp, err := http.Get(baseURL)
	if err != nil {
		logger.Printf("Error fetching URL %s: %v\n", baseURL, err)
		return results
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		logger.Printf("Error parsing HTML from %s: %v\n", baseURL, err)
		return results
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
		checksum, jsCode, err := getScriptChecksumAndContent(fullScriptURL)
		if err != nil {
			logger.Printf("Error processing script %s: %v\n", fullScriptURL, err)
			continue
		}
		
		logger.Printf("Found script: %s, Checksum: %s\n", fullScriptURL, checksum)

		libraryInfo := identifyLibrary(fullScriptURL, checksum, jsCode)
		if libraryInfo != nil {
			logger.Printf("Identified library for %s as: %s v%s (%s) [checksum: %s]\n", 
				fullScriptURL, libraryInfo.Name, libraryInfo.Version, libraryInfo.Method, libraryInfo.Checksum)
			
			result := ScanResult{
				URL:              baseURL,
				ScriptURL:        fullScriptURL,
				Checksum:         checksum,
				LibraryName:      libraryInfo.Name,
				LibraryVersion:   libraryInfo.Version,
				IdentifiedBy:     libraryInfo.Method,
			}
			results = append(results, result)
		}
	}
	
	return results
}

// resultCollector processes results as they come in
func (pp *ParallelProcessor) resultCollector(results <-chan URLResult, expectedCount int, done chan<- struct{}) {
	defer close(done)
	
	processedCount := 0
	
	for result := range results {
		processedCount++
		
		// Store scan results in database
		if pp.config.UseDB && len(result.ScanResults) > 0 {
			for _, scanResult := range result.ScanResults {
				if err := storeResult(scanResult); err != nil {
					logger.Printf("Error storing result for %s: %v\n", scanResult.ScriptURL, err)
				}
			}
		}
		
		// Update progress display
		pp.updateProgressDisplay(result)
		
		// Check if we're done
		if processedCount >= expectedCount {
			break
		}
	}
}

// updateProgressDisplay updates the progress display thread-safely
func (pp *ParallelProcessor) updateProgressDisplay(result URLResult) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	
	processed, _, _, _, _ := pp.tracker.GetCounts()
	
	if pp.config.Verbose {
		// Verbose output for each result
		fmt.Printf("\nProcessing URL: %s\n", result.Job.URL)
		
		if result.Excluded {
			fmt.Printf("  - Skipping excluded URL (Microsoft login domain)\n")
		} else if result.Error != nil {
			fmt.Printf("  - Error checking reachability: %v\n", result.Error)
		} else if result.Reachability != nil {
			if result.Reachability.HTTPAvailable || result.Reachability.HTTPSAvailable {
				protocols := []string{}
				if result.Reachability.HTTPAvailable {
					protocols = append(protocols, fmt.Sprintf("HTTP (%d)", result.Reachability.HTTPStatusCode))
				}
				if result.Reachability.HTTPSAvailable {
					protocols = append(protocols, fmt.Sprintf("HTTPS (%d)", result.Reachability.HTTPSStatusCode))
				}
				fmt.Printf("  - Reachable via: %s\n", strings.Join(protocols, ", "))
				
				if result.Reachability.HTTPRedirectURL != "" || result.Reachability.HTTPSRedirectURL != "" {
					fmt.Printf("  - Redirects detected\n")
				}
				
				if result.Reachability.FinalURL != "" && result.Reachability.FinalURL != result.Job.URL {
					fmt.Printf("  - Final URL: %s\n", result.Reachability.FinalURL)
				}
				
				if result.Skipped {
					fmt.Printf("  - Skipping JavaScript scan (no HTTP 200 response)\n")
				} else if len(result.ScanResults) > 0 {
					fmt.Printf("  - Scanning for JavaScript libraries...\n")
					for _, scanResult := range result.ScanResults {
						if scanResult.LibraryVersion != "unknown" && scanResult.LibraryVersion != "" {
							fmt.Printf("    Library: %s v%s (%s) [%s...]\n", 
								scanResult.LibraryName, scanResult.LibraryVersion, 
								scanResult.IdentifiedBy, scanResult.Checksum[:8])
						} else {
							fmt.Printf("    Library: %s (%s) [%s...]\n", 
								scanResult.LibraryName, scanResult.IdentifiedBy, scanResult.Checksum[:8])
						}
					}
				}
			} else {
				fmt.Printf("  - URL not reachable\n")
			}
		}
	} else {
		// Non-verbose progress indicator
		if !result.Excluded && !result.Skipped && result.Error == nil && len(result.ScanResults) >= 0 {
			finalURL := result.Job.URL
			if result.Reachability != nil && result.Reachability.FinalURL != "" {
				finalURL = result.Reachability.FinalURL
			}
			
			fmt.Printf("\n[%d/%d] Scanning: %s → %d scripts found", 
				processed, pp.tracker.total, finalURL, len(result.ScanResults))
			fmt.Print("\nProgress: ")
		}
		
		// Progress dots
		if processed%10 == 0 || processed == pp.tracker.total {
			fmt.Printf(" %d", processed)
		} else {
			fmt.Print(".")
		}
	}
}

// displayFinalSummary displays the final summary
func (pp *ParallelProcessor) displayFinalSummary() {
	if !pp.config.Verbose {
		processed, scanned, excluded, skipped, errors := pp.tracker.GetCounts()
		
		pp.mu.Lock()
		fmt.Printf("\n\nScan completed!\n")
		fmt.Printf("Total URLs processed: %d\n", processed)
		fmt.Printf("Successfully scanned: %d\n", scanned)
		if excluded > 0 {
			fmt.Printf("Excluded URLs: %d\n", excluded)
		}
		if skipped > 0 {
			fmt.Printf("Skipped (non-200): %d\n", skipped)
		}
		if errors > 0 {
			fmt.Printf("Errors/Unreachable: %d\n", errors)
		}
		pp.mu.Unlock()
	}
}