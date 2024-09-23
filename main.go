package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

func executeCommand(commandLine string, wait int64) (string, error) {
	// Split the command-line into executable and arguments
	cmdParts := strings.Fields(commandLine) // Split the string into command and arguments

	// The first part should be the executable (or script path)
	executable := cmdParts[0]
	args := cmdParts[1:]

	// Directly invoke the executable with arguments
	cmd := exec.Command(executable, args...)

	// If 'wait' is 1, run the command and wait for it to finish
	if wait == 1 {
		output, err := cmd.CombinedOutput() // Wait for command to finish and get output
		return string(output), err
	}

	// 'wait' is 0, start the command and return immediately
	if err := cmd.Start(); err != nil {
		return "", err
	}
	// Return immediately without waiting for the command to finish
	return "Command started successfully", nil
}

type executeCommandRequest struct {
	CommandLine string `json:"CommandLine"` // Full command-line string including the path and parameters
	Wait        int64  `json:"Wait"`        // Indicator for if the service should wait for the command to complete or not (1 vs 0 respectively)
}

// commandHandler is the handler for the HTTP request
func commandHandler(w http.ResponseWriter, r *http.Request) {
	var commandLine string
	var wait int64 = -1

	// We accept calling this service as a function (GET) as well as an action (POST)
	switch r.Method {
	case http.MethodGet:
		// Grab query parameters
		q := r.URL.Query()

		// Retrieve the 'commandLine' query parameter
		commandLine = q.Get("CommandLine")

		// Retrieve the 'wait' query parameter and convert it to boolean
		waitParam := q.Get("Wait")
		if waitParam != "" {
			parsedWait, err := strconv.ParseInt(waitParam, 10, 0)
			if err == nil {
				wait = parsedWait
			}
		}

	case http.MethodPost:
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		// Parse the JSON request into the CommandRequest struct
		var cmdReq executeCommandRequest
		if err := json.Unmarshal(body, &cmdReq); err != nil {
			http.Error(w, "Invalid JSON format", http.StatusBadRequest)
			return
		}

		// Use the command-line and wait parameter specified in the JSON payload
		commandLine = cmdReq.CommandLine
		wait = cmdReq.Wait

	default:
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Ensure the command line is provided
	if commandLine == "" {
		http.Error(w, "No or invalid CommandLine specified", http.StatusBadRequest)
		return
	}

	// Ensure wait parameter is provided
	if wait != 0 && wait != 1 {
		http.Error(w, "No or invalid Wait value specified", http.StatusBadRequest)
		return
	}

	// Execute the ExecuteCommand
	output, err := executeCommand(commandLine, wait)
	if err != nil {
		if output != "" {
			http.Error(w, fmt.Sprintf("Error executing command: %v\nOutput: %s", err, output), http.StatusInternalServerError)
		} else {
			http.Error(w, fmt.Sprintf("Error executing command: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Respond with the command output
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(output))
}

var elog debug.Log

type executeCommandService struct {
	port string
}

func (m *executeCommandService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	go runServer(m.port)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			break loop
		default:
			elog.Error(1, string(c.Cmd))
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runWindowsService(name string, port string) {
	run := svc.Run
	elog.Info(1, "starting "+name+" service on port "+port)
	err := run(name, &executeCommandService{port})
	if err != nil {
		elog.Error(1, "service "+name+" failed: "+err.Error())
		return
	}
	elog.Info(1, "service "+name+" stopped")
}

func runServer(port string) {
	// Setup the HTTP server and route
	http.HandleFunc("/ExecuteCommand", commandHandler)

	// Start listening to the user-defined or default port
	http.ListenAndServe(":"+port, nil)
}

func main() {
	// Define a command-line flag for the port, with a default value of 8080
	port := flag.String("port", "8080", "Port for the HTTP server to listen on")
	flag.Parse() // Parse the command-line flags

	// Determine if the service is begin started as a Windows service
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("Failed to determine if we are running in a windows service: %v", err)
	}

	// Start the ExecuteCommand service
	if isWindowsService {
		elog, err = eventlog.Open("ExecuteCommandService")
		if err != nil {
			return
		}
		defer elog.Close()
		runWindowsService("ExecuteCommandService", *port)
	} else {
		fmt.Printf("Starting ExecuteCommand service on port %s...\n", *port)
		elog = debug.New("ExecuteCommandService")
		runServer(*port)
	}
}
