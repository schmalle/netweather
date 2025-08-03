package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

// NmapScanRequest represents a request to the nmap scanner service
type NmapScanRequest struct {
	URLs    []string `json:"urls"`
	Ports   string   `json:"ports,omitempty"`
	Options string   `json:"options,omitempty"`
}

// NmapBatchResponse represents the response from creating a batch
type NmapBatchResponse struct {
	BatchID string `json:"batch_id"`
}

// NmapBatchStatus represents the status of a batch scan
type NmapBatchStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	URLs      []string  `json:"urls"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Progress  int       `json:"progress"`
	Results   string    `json:"results,omitempty"`
}

// NmapResult represents parsed nmap scan results
type NmapResult struct {
	URL       string
	IP        string
	Hostname  string
	OpenPorts []PortInfo
	ScanTime  time.Time
}

// PortInfo represents information about an open port
type PortInfo struct {
	Port     string
	Protocol string
	State    string
	Service  string
	Product  string
	Version  string
}

// NmapRun represents the root element of nmap XML output
type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Hosts   []Host   `xml:"host"`
}

// Host represents a scanned host
type Host struct {
	XMLName   xml.Name  `xml:"host"`
	Addresses []Address `xml:"address"`
	Ports     Ports     `xml:"ports"`
	Hostnames Hostnames `xml:"hostnames"`
}

// Address represents an IP address
type Address struct {
	XMLName  xml.Name `xml:"address"`
	Addr     string   `xml:"addr,attr"`
	AddrType string   `xml:"addrtype,attr"`
}

// Ports contains port information
type Ports struct {
	XMLName xml.Name `xml:"ports"`
	Ports   []Port   `xml:"port"`
}

// Port represents a single port
type Port struct {
	XMLName  xml.Name `xml:"port"`
	Protocol string   `xml:"protocol,attr"`
	PortID   string   `xml:"portid,attr"`
	State    State    `xml:"state"`
	Service  Service  `xml:"service"`
}

// State represents port state
type State struct {
	XMLName xml.Name `xml:"state"`
	State   string   `xml:"state,attr"`
	Reason  string   `xml:"reason,attr"`
}

// Service represents service information
type Service struct {
	XMLName xml.Name `xml:"service"`
	Name    string   `xml:"name,attr"`
	Product string   `xml:"product,attr"`
	Version string   `xml:"version,attr"`
}

// Hostnames contains hostname information
type Hostnames struct {
	XMLName   xml.Name   `xml:"hostnames"`
	Hostnames []Hostname `xml:"hostname"`
}

// Hostname represents a hostname
type Hostname struct {
	XMLName xml.Name `xml:"hostname"`
	Name    string   `xml:"name,attr"`
	Type    string   `xml:"type,attr"`
}

const (
	nmapServiceURL = "http://localhost:8080" // Default nmap service URL
)

// performPortScan performs port scanning for a given URL
func performPortScan(targetURL, ports, options string) {
	// Extract hostname/IP from URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		logger.Printf("Error parsing URL %s: %v", targetURL, err)
		fmt.Printf("    Error: Invalid URL format\n")
		return
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		logger.Printf("Error: No hostname found in URL %s", targetURL)
		fmt.Printf("    Error: No hostname found\n")
		return
	}

	// Check if Docker container is running
	if !isNmapServiceRunning() {
		logger.Printf("NMAP service not running, starting Docker container...")
		fmt.Printf("    Starting NMAP scanner container...\n")
		
		if err := startNmapContainer(); err != nil {
			logger.Printf("Error starting NMAP container: %v", err)
			fmt.Printf("    Error: Failed to start NMAP container\n")
			return
		}
		
		// Wait for service to be ready
		if !waitForNmapService(30 * time.Second) {
			logger.Printf("NMAP service failed to start")
			fmt.Printf("    Error: NMAP service failed to start\n")
			return
		}
	}

	// Create scan request
	scanReq := NmapScanRequest{
		URLs:    []string{hostname},
		Ports:   ports,
		Options: options,
	}

	// Send scan request
	batchID, err := createNmapBatch(scanReq)
	if err != nil {
		logger.Printf("Error creating NMAP batch: %v", err)
		fmt.Printf("    Error: Failed to create scan batch\n")
		return
	}

	logger.Printf("Created NMAP batch %s for %s", batchID, hostname)
	fmt.Printf("    Scan batch created: %s\n", batchID)

	// Store batch ID for later retrieval
	storeBatchID(batchID, targetURL)

	// Wait for scan completion (with timeout)
	timeout := 5 * time.Minute
	if err := waitForBatchCompletion(batchID, timeout); err != nil {
		logger.Printf("Batch %s did not complete: %v", batchID, err)
		fmt.Printf("    Scan timeout or error (batch: %s)\n", batchID)
		return
	}

	// Retrieve and process results
	results, err := getNmapResults(batchID)
	if err != nil {
		logger.Printf("Error retrieving results for batch %s: %v", batchID, err)
		fmt.Printf("    Error retrieving scan results\n")
		return
	}

	// Parse and display results
	nmapResults, err := parseNmapXML(results)
	if err != nil {
		logger.Printf("Error parsing NMAP results: %v", err)
		fmt.Printf("    Error parsing scan results\n")
		return
	}

	displayNmapResults(nmapResults, targetURL)
}

// isNmapServiceRunning checks if the nmap service is accessible
func isNmapServiceRunning() bool {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(nmapServiceURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// startNmapContainer starts the nmap scanner Docker container
func startNmapContainer() error {
	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("Docker is not installed or not in PATH")
	}

	// Build the Docker image if it doesn't exist
	buildCmd := exec.Command("docker", "build", "-t", "netweather-nmap", "./docker/nmap-scanner")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		logger.Printf("Docker build output: %s", output)
		return fmt.Errorf("failed to build Docker image: %v", err)
	}

	// Run the container
	runCmd := exec.Command("docker", "run", "-d", "--name", "netweather-nmap-scanner", 
		"-p", "8080:8080", "--rm", "netweather-nmap")
	if output, err := runCmd.CombinedOutput(); err != nil {
		logger.Printf("Docker run output: %s", output)
		return fmt.Errorf("failed to start Docker container: %v", err)
	}

	return nil
}

// waitForNmapService waits for the nmap service to become ready
func waitForNmapService(timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if isNmapServiceRunning() {
				return true
			}
		}
	}
}

// createNmapBatch creates a new scan batch
func createNmapBatch(req NmapScanRequest) (string, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(nmapServiceURL+"/batch", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	var batchResp NmapBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return "", err
	}

	return batchResp.BatchID, nil
}

// waitForBatchCompletion waits for a batch to complete
func waitForBatchCompletion(batchID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for batch completion")
		case <-ticker.C:
			status, err := getNmapBatchStatus(batchID)
			if err != nil {
				continue
			}

			switch status.Status {
			case "completed":
				return nil
			case "failed":
				return fmt.Errorf("batch failed: %s", status.Results)
			}
		}
	}
}

// getNmapBatchStatus gets the status of a batch
func getNmapBatchStatus(batchID string) (*NmapBatchStatus, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/batch/%s", nmapServiceURL, batchID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	var status NmapBatchStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return &status, nil
}

// getNmapResults retrieves the XML results for a completed batch
func getNmapResults(batchID string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/batch/%s/results", nmapServiceURL, batchID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	return io.ReadAll(resp.Body)
}

// parseNmapXML parses nmap XML results
func parseNmapXML(xmlData []byte) ([]NmapResult, error) {
	var nmapRun NmapRun
	if err := xml.Unmarshal(xmlData, &nmapRun); err != nil {
		return nil, err
	}

	var results []NmapResult
	for _, host := range nmapRun.Hosts {
		result := NmapResult{
			ScanTime: time.Now(),
		}

		// Get IP address
		for _, addr := range host.Addresses {
			if addr.AddrType == "ipv4" {
				result.IP = addr.Addr
				break
			}
		}

		// Get hostname
		if len(host.Hostnames.Hostnames) > 0 {
			result.Hostname = host.Hostnames.Hostnames[0].Name
		}

		// Get open ports
		for _, port := range host.Ports.Ports {
			if port.State.State == "open" {
				portInfo := PortInfo{
					Port:     port.PortID,
					Protocol: port.Protocol,
					State:    port.State.State,
					Service:  port.Service.Name,
					Product:  port.Service.Product,
					Version:  port.Service.Version,
				}
				result.OpenPorts = append(result.OpenPorts, portInfo)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// displayNmapResults displays the parsed nmap results
func displayNmapResults(results []NmapResult, originalURL string) {
	for _, result := range results {
		fmt.Printf("    Port scan results for %s:\n", originalURL)
		if result.IP != "" {
			fmt.Printf("      IP: %s\n", result.IP)
		}
		if result.Hostname != "" {
			fmt.Printf("      Hostname: %s\n", result.Hostname)
		}

		if len(result.OpenPorts) == 0 {
			fmt.Printf("      No open ports found\n")
		} else {
			fmt.Printf("      Open ports:\n")
			for _, port := range result.OpenPorts {
				serviceInfo := port.Service
				if port.Product != "" {
					serviceInfo += fmt.Sprintf(" (%s", port.Product)
					if port.Version != "" {
						serviceInfo += fmt.Sprintf(" %s", port.Version)
					}
					serviceInfo += ")"
				}
				fmt.Printf("        %s/%s - %s\n", port.Port, port.Protocol, serviceInfo)
			}
		}
	}
}

// storeBatchID stores a batch ID for later retrieval
func storeBatchID(batchID, url string) {
	// Store in database if available
	if db != nil {
		query := "INSERT INTO nmap_batches (batch_id, url, status, created_at) VALUES (?, ?, ?, ?)"
		_, err := db.Exec(query, batchID, url, "running", time.Now())
		if err != nil {
			logger.Printf("Error storing batch ID: %v", err)
		}
	}
	logger.Printf("Stored batch ID %s for URL %s", batchID, url)
}