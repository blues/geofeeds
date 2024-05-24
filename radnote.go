// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sync"

	"github.com/blues/note-go/note"
)

// The body of a Radnote-generated event
type RadnoteEventBody struct {
	Cpm          float64 `json:"cpm,omitempty"`
	CpmCount     int     `json:"cpm_count,omitempty"`
	CpmCountSecs int     `json:"csecs,omitempty"`
	Sensor       string  `json:"sensor,omitempty"`
	TemperatureC float64 `json:"temperature,omitempty"`
	Voltage      float64 `json:"voltage,omitempty"`
	Usv          float64 `json:"usv,omitempty"`
}

// An Event with a Radnote-specific body type added
type RadnoteEvent struct {
	Event note.Event       `json:"event,omitempty"`
	Body  RadnoteEventBody `json:"body,omitempty"`
}

// Loaded radnote data
var radnoteLock sync.Mutex
var radnoteEvents map[string]RadnoteEvent
var radnoteFile = "radnote.json"

// Radnote event handler
func httpRadnoteHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Make sure the data is loaded
	radnoteLock.Lock()
	if radnoteEvents == nil {
		radnoteEvents = map[string]RadnoteEvent{}
		contents, err := os.ReadFile(configDataDirectory + radnoteFile)
		if err == nil {
			err = note.JSONUnmarshal(contents, &radnoteEvents)
			if err != nil {
				fmt.Printf("radnote: can't load %s: %s\n", radnoteFile, err)
			}
		}
	}
	radnoteLock.Unlock()

	// If GET, return the results
	if r.Method == "GET" {
		w.WriteHeader(http.StatusOK)
		radnoteLock.Lock()
		var eventJSON []byte
		eventJSON, err = json.MarshalIndent(radnoteEvents, "", "    ")
		if err == nil {
			_, _ = w.Write(eventJSON)
		}
		return
	}

	// If not POST, we don't know why we're here
	if r.Method != "POST" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

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

	// Exit if not a data reading
	if event.NotefileID != "_air.qo" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Retain the last event and persist the body
	radnoteLock.Lock()
	currentEvent, exists := radnoteEvents[event.DeviceUID]
	if !exists || event.When >= currentEvent.Event.When {
		radevent := RadnoteEvent{}
		radevent.Event = event
		radevent.Event.Body = nil
		if event.Body != nil {
			bodyJSON, _ := note.JSONMarshal(*event.Body)
			_ = note.JSONUnmarshal(bodyJSON, &radevent.Body)
		}
		radnoteEvents[event.DeviceUID] = radevent
		eventJSON, err = json.Marshal(radnoteEvents)
		if err == nil {
			err = os.WriteFile(configDataDirectory+radnoteFile, eventJSON, 0644)
		}
	}
	radnoteLock.Unlock()
	if err != nil {
		fmt.Printf("radnote: can't store %s: %s\n", radnoteFile, err)
	}

	// For informational purposes, see if this radnote is within a warning zone
	radnoteLock.Lock()
	if radnoteInWarningRegion(event.DeviceUID) {
		fmt.Printf("WARNING: radnote %s is in warning region\n", event.DeviceUID)
	} else {
		fmt.Printf("radnote %s is in a safe region\n", event.DeviceUID)
	}
	radnoteLock.Unlock()

}

// See if a given radnote is within a warning region
func radnoteInWarningRegion(deviceUID string) bool {

	this, found := radnoteEvents[deviceUID]
	if !found {
		return false
	}

	for _, e := range radnoteEvents {
		fmt.Printf("eusv:%f configusv:%f dist:%f configdist:%f\n", e.Body.Usv, config.RadnoteWarningLevelUsv, metersApart(e.Event.BestLat, e.Event.BestLon, this.Event.BestLat, this.Event.BestLon), config.RadnoteWarningRegionMeters)
		if e.Body.Usv >= config.RadnoteWarningLevelUsv {
			if e.Event.BestLat != 0 || e.Event.BestLon != 0 {
				if metersApart(e.Event.BestLat, e.Event.BestLon, this.Event.BestLat, this.Event.BestLon) <= config.RadnoteWarningRegionMeters {
					return true
				}
			}
		}
	}

	return false
}

// Distance function returns the distance (in meters) between two points of
//
//	a given longitude and latitude relatively accurately (using a spherical
//	approximation of the Earth) through the Haversin Distance Formula for
//	great arc distance on a sphere with accuracy for small distances
//
// point coordinates are supplied in degrees and converted into rad. in the func
//
// distance returned is METERS
func metersApart(lat1 float64, lon1 float64, lat2 float64, lon2 float64) float64 {
	const earthRadiusMeters = 6378100
	const earthRadiusMetersDoubled = earthRadiusMeters * 2
	var la2, lo2 float64
	fmt.Printf("%f,%f -> %f,%f\n", lat1, lon1, lat2, lon2)

	// convert to radians
	// must cast radius as float to multiply later
	la2 = lat2 * math.Pi / 180
	lo2 = lon2 * math.Pi / 180

	// calculate
	h := hsin(la2-lat1) + math.Cos(lat1)*math.Cos(la2)*hsin(lo2-lon1)

	return earthRadiusMetersDoubled * math.Asin(math.Sqrt(h))

}

// haversin(θ) function
// http://en.wikipedia.org/wiki/Haversine_formula
func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}
