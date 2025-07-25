package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIResponse represents the response from the publicdata.guru API.
type APIResponse struct {
	Results []struct {
		Package struct {
			Name string `json:"name"`
		} `json:"package"`
	} `json:"results"`
}

// identifyLibrary queries the publicdata.guru API to identify a library by its checksum.
func identifyLibrary(checksum string) (string, error) {
	url := fmt.Sprintf("https://api.publicdata.guru/v1/checksums/%s", checksum)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	var apiResponse APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return "", err
	}

	if len(apiResponse.Results) > 0 {
		return apiResponse.Results[0].Package.Name, nil
	}

	return "Unknown", nil
}
