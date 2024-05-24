// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/blues/note-go/note"
)

// Radnote event handler
func httpRadnoteHandler(w http.ResponseWriter, r *http.Request) {

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

	fmt.Printf("%+v\n%s\n", event, string(eventJSON))

}
