package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config file
type Config struct {
	// Generate a alert at or above this uSV level
	RadnoteAlertLevelUsv float64 `json:"radnote_alert_at_usv,omitempty"`
	// Number of meters away if it's in a alert region
	RadnoteAlertRegionMeters float64 `json:"radnote_alert_region_meters,omitempty"`
	// Generate an alert for this many minutes
	RadnoteAlertMins int64 `json:"radnote_alert_minutes,omitempty"`
	// If an alert is active, sample with this period
	RadnoteAlertSampleMins int64 `json:"radnote_alert_sample_minutes,omitempty"`
	// If an alert is active, sync with this period
	RadnoteAlertSyncMins int64 `json:"radnote_alert_sync_minutes,omitempty"`
}

var config Config

// Fully-resolved data directory
var configDataDirectory = "/home/ubuntu" + "/data/"

// Load the config
func configLoad() {

	configPath := configDataDirectory + "config.json"
	contents, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("config: can't load from %s: %s\n", configPath, err)
		os.Exit(-1)
	}

	err = json.Unmarshal(contents, &config)
	if err != nil {
		fmt.Printf("config: can't parse JSON: %s\n%s\n", err, contents)
		os.Exit(-1)
	}

}
