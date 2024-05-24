// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/blues/note-go/note"
)

// Loaded radnote data
var radnoteLock sync.Mutex
var radnoteEvents map[string]note.Event
var radnoteFile = "radnote.json"

// Radnote event handler
func httpRadnoteHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Make sure the data is loaded
	radnoteLock.Lock()
	if radnoteEvents == nil {
		radnoteEvents = map[string]note.Event{}
		contents, err := os.ReadFile(configDataDirectory + radnoteFile)
		if err == nil {
			err = note.JSONUnmarshal(contents, &radnoteEvents)
			if err != nil {
				fmt.Printf("radnote: can't load %s: %s\n", radnoteFile, err)
			}
		}
	}
	radnoteLock.Unlock()

	// Get the event body
	eventJSON, err := io.ReadAll(r.Body)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	event := note.Event{}
	err = note.JSONUnmarshal(eventJSON, &event)
	if err != nil {
		_, _ = w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Retain the last event and persist the body
	radnoteLock.Lock()
	radnoteEvents[event.DeviceUID] = event
	eventJSON, err = json.Marshal(radnoteEvents)
	if err == nil {
		err = os.WriteFile(configDataDirectory+radnoteFile, eventJSON, 0644)
	}
	radnoteLock.Unlock()
	if err != nil {
		fmt.Printf("radnote: can't store %s: %s\n", radnoteFile, err)
	}

}
