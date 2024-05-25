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
	"strconv"
	"sync"
	"time"

	"github.com/blues/note-go/note"
	"github.com/kr/jsonfeed"
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

		// See if lat/lon are specified
		query := r.URL.Query()
		lat, latErr := strconv.ParseFloat(query.Get("lat"), 64)
		lon, lonErr := strconv.ParseFloat(query.Get("lon"), 64)
		fmt.Printf("QUERY: %+v\n", query)
		fmt.Printf("lat: %s %f %s\n", query.Get("lat"), lat, latErr)
		fmt.Printf("lon: %s %f %s\n", query.Get("lon"), lon, lonErr)
		if latErr == nil && lonErr == nil && !(lat == 0 && lon == 0) {
			generateJsonFeed(w, r, lat, lon)
			return
		}

		// Just retrieve the list
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

}

// See if a given radnote is within a warning region
func locationInWarningRegion(lat float64, lon float64) bool {

	// Lock during array scan
	radnoteLock.Lock()
	defer radnoteLock.Unlock()

	// See if this location is within a warning zone
	for _, e := range radnoteEvents {
		if e.Body.Usv >= config.RadnoteWarningLevelUsv {
			if e.Event.BestLat != 0 || e.Event.BestLon != 0 {
				if metersApart(e.Event.BestLat, e.Event.BestLon, lat, lon) <= config.RadnoteWarningRegionMeters {
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
func metersApart(lat1 float64, lon1 float64, lat2 float64, lon2 float64) (distanceMeters float64) {
	const R = 6371
	const degreesToRadians = (3.1415926536 / 180)
	var dx, dy, dz float64
	lon1 = lon1 - lon2
	lon1 = lon1 * degreesToRadians
	lat1 = lat1 * degreesToRadians
	lat2 = lat2 * degreesToRadians
	dz = math.Sin(lat1) - math.Sin(lat2)
	dx = math.Cos(lon1)*math.Cos(lat1) - math.Cos(lat2)
	dy = math.Sin(lon1) * math.Cos(lat1)
	distanceMeters = 1000 * (math.Asin(math.Sqrt(math.Abs(dx*dx+dy*dy+dz*dz))/2) * 2 * R)
	return
}

// Generate a JSON feed for the specified location
func generateJsonFeed(w http.ResponseWriter, r *http.Request, lat float64, lon float64) {

	o := map[string]interface{}{}
	if locationInWarningRegion(lat, lon) {
		o["warning"] = true
		o["sample_mins"] = 15
		o["outbound_mins"] = 60
	} else {
		o["warning"] = false
	}
	oJSON, err := json.Marshal(o)
	if err != nil {
		fmt.Printf("generateJsonFeed: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var i jsonfeed.Item
	i.ID = "1"
	i.URL = fmt.Sprintf("https://geofeeds.net/radnote/%s?lat=%f&lon=%f", i.ID, lat, lon)
	i.ContentText = string(oJSON)
	i.DatePublished = time.Now().UTC()

	var f jsonfeed.Feed
	f.Version = "https://jsonfeed.org/version/1"
	f.FeedURL = fmt.Sprintf("https://geofeeds.net/radnote/?lat=%f&lon=%f", lat, lon)
	f.Items = append(f.Items, i)

	feedJSON, err := f.MarshalJSON()
	if err != nil {
		fmt.Printf("generateJsonFeed: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(feedJSON)

}
