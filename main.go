package main

import (
	"encoding/json"
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

var elog debug.Log

type executeCommandService struct{}

func (m *executeCommandService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	go runServer()
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

func main() {
	isWindowsService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
	}
	if isWindowsService {
		elog, err = eventlog.Open("ExecuteCommandService")
		if err != nil {
			return
		}
		defer elog.Close()
		runWindowsService("ExecuteCommandService")
	} else {
		elog = debug.New("ExecuteCommandService")
		runServer()
	}
}

func runWindowsService(name string) {
	run := svc.Run
	elog.Info(1, "starting "+name+" service")
	err := run(name, &executeCommandService{})
	if err != nil {
		elog.Error(1, "service "+name+" failed: "+err.Error())
		return
	}
	elog.Info(1, "service "+name+" stopped")
}

func runServer() {
	http.HandleFunc("/ExecuteCommand", commandHandler)
	http.ListenAndServe(":8080", nil)
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
		http.Error(w, fmt.Sprintf("Error executing command: %v\nOutput: %s", err, output), http.StatusInternalServerError)
		return
	}

	// Respond with the command output
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(output))
}

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
