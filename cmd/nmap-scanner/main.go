package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/google/uuid"
)

// ScanRequest represents a scan request
type ScanRequest struct {
	URLs   []string `json:"urls"`
	Ports  string   `json:"ports,omitempty"`
	Options string  `json:"options,omitempty"`
}

// BatchStatus represents the status of a batch scan
type BatchStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // pending, running, completed, failed
	URLs      []string  `json:"urls"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Progress  int       `json:"progress"` // percentage
	Results   string    `json:"results,omitempty"`
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
	XMLName xml.Name `xml:"address"`
	Addr    string   `xml:"addr,attr"`
	AddrType string  `xml:"addrtype,attr"`
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

var (
	batches = make(map[string]*BatchStatus)
	batchesDir = "/app/batches"
	resultsDir = "/app/results"
)

func main() {
	// Create directories if they don't exist
	os.MkdirAll(batchesDir, 0755)
	os.MkdirAll(resultsDir, 0755)

	// Load existing batches from disk
	loadBatches()

	r := mux.NewRouter()
	
	// API endpoints
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/scan", scanSingleURLHandler).Methods("POST")
	r.HandleFunc("/batch", createBatchHandler).Methods("POST")
	r.HandleFunc("/batch/{id}", getBatchStatusHandler).Methods("GET")
	r.HandleFunc("/batch/{id}/results", getBatchResultsHandler).Methods("GET")
	r.HandleFunc("/batches", listBatchesHandler).Methods("GET")

	log.Println("NMAP Scanner Service starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func scanSingleURLHandler(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.URLs) != 1 {
		http.Error(w, "Single URL scan requires exactly one URL", http.StatusBadRequest)
		return
	}

	// Create a temporary batch for single URL scan
	batchID := uuid.New().String()
	batch := &BatchStatus{
		ID:        batchID,
		Status:    "running",
		URLs:      req.URLs,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Progress:  0,
	}

	// Run scan synchronously for single URL
	go runScan(batch, req.Ports, req.Options)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"batch_id": batchID})
}

func createBatchHandler(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.URLs) == 0 {
		http.Error(w, "No URLs provided", http.StatusBadRequest)
		return
	}

	batchID := uuid.New().String()
	batch := &BatchStatus{
		ID:        batchID,
		Status:    "pending",
		URLs:      req.URLs,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Progress:  0,
	}

	batches[batchID] = batch
	saveBatch(batch)

	// Start scan in background
	go runScan(batch, req.Ports, req.Options)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"batch_id": batchID})
}

func getBatchStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	batchID := vars["id"]

	batch, exists := batches[batchID]
	if !exists {
		http.Error(w, "Batch not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(batch)
}

func getBatchResultsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	batchID := vars["id"]

	batch, exists := batches[batchID]
	if !exists {
		http.Error(w, "Batch not found", http.StatusNotFound)
		return
	}

	if batch.Status != "completed" {
		http.Error(w, "Batch not completed yet", http.StatusBadRequest)
		return
	}

	resultsPath := filepath.Join(resultsDir, batchID+".xml")
	file, err := os.Open(resultsPath)
	if err != nil {
		http.Error(w, "Results not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s-results.xml\"", batchID))
	io.Copy(w, file)
}

func listBatchesHandler(w http.ResponseWriter, r *http.Request) {
	var batchList []*BatchStatus
	for _, batch := range batches {
		batchList = append(batchList, batch)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(batchList)
}

func runScan(batch *BatchStatus, ports, options string) {
	batch.Status = "running"
	batch.UpdatedAt = time.Now()
	saveBatch(batch)

	resultsPath := filepath.Join(resultsDir, batch.ID+".xml")
	
	// Build nmap command
	args := []string{"-oX", resultsPath}
	
	// Add port specification if provided
	if ports != "" {
		args = append(args, "-p", ports)
	} else {
		args = append(args, "-p", "80,443,8080,8443") // Default common web ports
	}
	
	// Add custom options if provided
	if options != "" {
		optionList := strings.Fields(options)
		args = append(args, optionList...)
	} else {
		// Default safe options
		args = append(args, "-sS", "-sV", "--script=default,safe")
	}
	
	// Add URLs
	args = append(args, batch.URLs...)

	log.Printf("Running nmap with args: %v", args)
	
	cmd := exec.Command("nmap", args...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		log.Printf("Nmap scan failed: %v, output: %s", err, output)
		batch.Status = "failed"
		batch.Results = fmt.Sprintf("Scan failed: %v", err)
	} else {
		batch.Status = "completed"
		batch.Progress = 100
		log.Printf("Nmap scan completed for batch %s", batch.ID)
	}
	
	batch.UpdatedAt = time.Now()
	saveBatch(batch)
}

func saveBatch(batch *BatchStatus) {
	batchPath := filepath.Join(batchesDir, batch.ID+".json")
	data, err := json.Marshal(batch)
	if err != nil {
		log.Printf("Error marshaling batch %s: %v", batch.ID, err)
		return
	}
	
	err = os.WriteFile(batchPath, data, 0644)
	if err != nil {
		log.Printf("Error saving batch %s: %v", batch.ID, err)
	}
}

func loadBatches() {
	files, err := filepath.Glob(filepath.Join(batchesDir, "*.json"))
	if err != nil {
		log.Printf("Error loading batches: %v", err)
		return
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Error reading batch file %s: %v", file, err)
			continue
		}

		var batch BatchStatus
		if err := json.Unmarshal(data, &batch); err != nil {
			log.Printf("Error unmarshaling batch file %s: %v", file, err)
			continue
		}

		batches[batch.ID] = &batch
		log.Printf("Loaded batch %s with status %s", batch.ID, batch.Status)
	}
}