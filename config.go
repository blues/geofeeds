package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config file
type Config struct {
}

var config Config

// Fully-resolved data directory
var configDataDirectory = os.Getenv("HOME") + "/data/"

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
