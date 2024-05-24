// Copyright 2024 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Main service entry point
func main() {

	// Register root endpoint
	http.HandleFunc("/", httpRootHandler)

	// Register AWS health check endpoint
	http.HandleFunc("/ping", httpPingHandler)
	go func() { _ = http.ListenAndServe(":80", nil) }()

	// Register radnote endpoint
	http.HandleFunc("/radnote", httpRadnoteHandler)

	// Spawn our signal handler
	go signalHandler()

	// Handle console input so we can manually quit and relaunch
	inputHandler()

}

// Root handler
func httpRootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" && r.URL.Path == "/favicon.ico" {
		w.WriteHeader(http.StatusOK)
	}
	w.WriteHeader(http.StatusNotImplemented)
}

// Ping handler, for AWS health checks
func httpPingHandler(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(time.Now().UTC().Format("2006-01-02T15:04:05Z")))
}

func inputHandler() {

	scanner := bufio.NewScanner(os.Stdin)

	for {

		scanner.Scan()
		message := scanner.Text()

		args := strings.Split(message, " ")

		switch args[0] {
		case "q":
			os.Exit(0)
		case "":
			// just re-prompt
		default:
			fmt.Printf("Unrecognized: '%s'\n", message)
		}

		fmt.Print("\n> ")

	}

}

// Our app's signal handler
func signalHandler() {
	ch := make(chan os.Signal, 100)
	signal.Notify(ch, syscall.SIGTERM)
	signal.Notify(ch, syscall.SIGINT)
	signal.Notify(ch, syscall.SIGSEGV)
	for {
		switch <-ch {
		case syscall.SIGINT:
			fmt.Printf("*** Exiting because of SIGNAL \n")
			os.Exit(0)
		case syscall.SIGTERM:
			return
		}
	}
}
