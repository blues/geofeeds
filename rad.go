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
type RadEvent struct {
	Event note.Event `json:"event,omitempty"`
	Usv   float64    `json:"usv,omitempty"`
}

// Loaded radnote data
var radLock sync.Mutex
var radEvents map[string]RadEvent
var radFile = "rad.json"

// First time load of data
func ensureLoaded() {
	radLock.Lock()
	if radEvents == nil {
		radEvents = map[string]RadEvent{}
		contents, err := os.ReadFile(configDataDirectory + radFile)
		if err == nil {
			err = note.JSONUnmarshal(contents, &radEvents)
			if err != nil {
				fmt.Printf("radnote: can't load %s: %s\n", radFile, err)
			}
		}
	}
	radLock.Unlock()
}

// Radnote event handler
func httpRadnoteHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Make sure the data is loaded
	ensureLoaded()

	// Get the event body
	eventJSON, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("radnote: error reading POSTed body: %s\n", err)
		_, _ = w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	event := note.Event{}
	err = note.JSONUnmarshal(eventJSON, &event)
	if err != nil {
		fmt.Printf("radnote: error marshaling POSTed body: %s\n%s\n", err, eventJSON)
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
	radLock.Lock()
	currentEvent, exists := radEvents[event.DeviceUID]
	if !exists || event.When >= currentEvent.Event.When {
		radevent := RadEvent{}
		radevent.Event = event
		radevent.Event.Body = nil
		if event.Body != nil {
			bodyJSON, _ := note.JSONMarshal(*event.Body)
			var rev RadnoteEventBody
			_ = note.JSONUnmarshal(bodyJSON, &rev)
			radevent.Usv = rev.Usv
		}
		radEvents[event.DeviceUID] = radevent
		eventJSON, err = json.Marshal(radEvents)
		if err == nil {
			err = os.WriteFile(configDataDirectory+radFile, eventJSON, 0644)
		}
	}
	radLock.Unlock()
	if err != nil {
		fmt.Printf("radnote: can't store %s: %s\n", radFile, err)
	}

}

// Radiation query handler
func httpRadiationHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	// Make sure the data is loaded
	ensureLoaded()

	// See if lat/lon are specified, and if so, generate a feed
	query := r.URL.Query()
	latStr := query.Get("lat")
	lonStr := query.Get("lon")
	radiusMetersStr := query.Get("radius_meters")
	if latStr != "" && lonStr != "" {
		lat, latErr := strconv.ParseFloat(latStr, 64)
		lon, lonErr := strconv.ParseFloat(lonStr, 64)
		radiusMeters, radiusErr := strconv.ParseFloat(radiusMetersStr, 64)
		if latErr == nil && lonErr == nil && radiusErr == nil && !(lat == 0 && lon == 0) {
			generateJsonFeed(w, r, lat, lon, radiusMeters)
			return
		}
	}

	// Just retrieve the full list
	w.WriteHeader(http.StatusOK)
	var eventJSON []byte
	radLock.Lock()
	eventJSON, err = json.MarshalIndent(radEvents, "", "    ")
	radLock.Unlock()
	if err == nil {
		_, _ = w.Write(eventJSON)
	}
	return

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
func generateJsonFeed(w http.ResponseWriter, r *http.Request, lat float64, lon float64, radiusMeters float64) {

	// If 0, make it a small region
	if radiusMeters == 0 {
		radiusMeters = 10
	}

	// See if this location is within the region
	count := float64(0)
	min := float64(0)
	max := float64(0)
	sum := float64(0)
	radLock.Lock()
	for _, e := range radEvents {
		if e.Event.BestLat != 0 || e.Event.BestLon != 0 {
			if metersApart(e.Event.BestLat, e.Event.BestLon, lat, lon) <= radiusMeters {
				if count == 0 {
					min = e.Usv
					max = e.Usv
				}
				if e.Usv < min {
					min = e.Usv
				}
				if e.Usv > max {
					max = e.Usv
				}
				sum += e.Usv
				count++
			}
		}
	}
	radLock.Unlock()
	avg := float64(0)
	if count > 0 {
		avg = sum / count
	}

	// debug
	if count == 0 {
		fmt.Printf("NOT FOUND: %f,%f %f\n", lat, lon, radiusMeters)
		radLock.Lock()
		fmt.Printf("%+v\n", radEvents)
		radLock.Unlock()
	}
	// debug

	o := map[string]interface{}{}
	o["lat"] = lat
	o["lon"] = lon
	o["radius_meters"] = radiusMeters
	o["count"] = count
	o["usv_min"] = min
	o["usv_max"] = max
	o["usv_avg"] = avg
	o["modified"] = time.Now().UTC().Unix()
	oJSON, err := json.Marshal(o)
	if err != nil {
		fmt.Printf("generateJsonFeed: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var i jsonfeed.Item
	i.ID = "region"
	i.URL = fmt.Sprintf("https://geofeeds.net/radnote/%s?lat=%f&lon=%f", i.ID, lat, lon)
	i.ContentText = string(oJSON)
	i.DatePublished = time.Now().UTC()
	i.DateModified = i.DatePublished

	var f jsonfeed.Feed
	f.Version = "https://jsonfeed.org/version/1"
	f.Title = fmt.Sprintf("radnote geofeed for %f,%f", lat, lon)
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
