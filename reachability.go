package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// URLReachability holds the reachability information for a URL
type URLReachability struct {
	OriginalURL     string
	HTTPAvailable   bool
	HTTPSAvailable  bool
	HTTPStatusCode  int
	HTTPSStatusCode int
	HTTPRedirectURL string
	HTTPSRedirectURL string
	FinalURL        string
	ScannedAt       time.Time
}

// HasSuccessfulResponse returns true if the URL returned HTTP 200 on either protocol
func (r *URLReachability) HasSuccessfulResponse() bool {
	return (r.HTTPAvailable && r.HTTPStatusCode == 200) || 
	       (r.HTTPSAvailable && r.HTTPSStatusCode == 200)
}

// GetBestProtocol returns the best protocol to use (HTTPS preferred if both return 200)
func (r *URLReachability) GetBestProtocol() string {
	if r.HTTPSAvailable && r.HTTPSStatusCode == 200 {
		return "HTTPS"
	}
	if r.HTTPAvailable && r.HTTPStatusCode == 200 {
		return "HTTP"
	}
	return ""
}

// checkURLReachability checks if a URL is reachable via HTTP and/or HTTPS
func checkURLReachability(inputURL string) (*URLReachability, error) {
	result := &URLReachability{
		OriginalURL: inputURL,
		ScannedAt:   time.Now(),
	}
	
	// Create HTTP client with timeout and redirect handling
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	// Parse the input URL to determine if it has a scheme
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}
	
	// If no scheme is provided, we'll test both HTTP and HTTPS
	if parsedURL.Scheme == "" {
		// Clean the URL to ensure it doesn't start with //
		cleanURL := strings.TrimPrefix(inputURL, "//")
		
		// Check HTTP
		httpURL := "http://" + cleanURL
		checkProtocol(client, httpURL, result, true)
		
		// Check HTTPS
		httpsURL := "https://" + cleanURL
		checkProtocol(client, httpsURL, result, false)
		
		// Determine the final URL based on availability and preference
		if result.HTTPSAvailable {
			result.FinalURL = determineRedirectURL(httpsURL, result.HTTPSRedirectURL)
		} else if result.HTTPAvailable {
			result.FinalURL = determineRedirectURL(httpURL, result.HTTPRedirectURL)
		}
		
	} else {
		// URL has a scheme, check only that specific protocol
		if parsedURL.Scheme == "http" {
			checkProtocol(client, inputURL, result, true)
			result.FinalURL = determineRedirectURL(inputURL, result.HTTPRedirectURL)
		} else if parsedURL.Scheme == "https" {
			checkProtocol(client, inputURL, result, false)
			result.FinalURL = determineRedirectURL(inputURL, result.HTTPSRedirectURL)
		} else {
			return nil, fmt.Errorf("unsupported scheme: %s", parsedURL.Scheme)
		}
	}
	
	return result, nil
}

// checkProtocol checks a specific protocol (HTTP or HTTPS) for a URL
func checkProtocol(client *http.Client, url string, result *URLReachability, isHTTP bool) {
	logger.Printf("Checking reachability for %s\n", url)
	
	resp, err := client.Get(url)
	if err != nil {
		logger.Printf("Error checking %s: %v\n", url, err)
		return
	}
	defer resp.Body.Close()
	
	// Record the status code and availability
	if isHTTP {
		result.HTTPAvailable = true
		result.HTTPStatusCode = resp.StatusCode
		
		// Check if there was a redirect
		if resp.Request.URL.String() != url {
			result.HTTPRedirectURL = resp.Request.URL.String()
			logger.Printf("HTTP redirect from %s to %s\n", url, result.HTTPRedirectURL)
		}
	} else {
		result.HTTPSAvailable = true
		result.HTTPSStatusCode = resp.StatusCode
		
		// Check if there was a redirect
		if resp.Request.URL.String() != url {
			result.HTTPSRedirectURL = resp.Request.URL.String()
			logger.Printf("HTTPS redirect from %s to %s\n", url, result.HTTPSRedirectURL)
		}
	}
}

// determineRedirectURL returns the redirect URL if available, otherwise the original URL
func determineRedirectURL(originalURL, redirectURL string) string {
	if redirectURL != "" {
		return redirectURL
	}
	return originalURL
}

// checkAndFollowRedirects checks a URL and follows redirects to get the final URL
func checkAndFollowRedirects(inputURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	resp, err := client.Get(inputURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	// Return the final URL after redirects
	return resp.Request.URL.String(), nil
}