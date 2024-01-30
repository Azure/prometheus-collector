package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var inotify_config_output_file = "/opt/inotifyoutput-ta-config.txt"

// func monitorInotify(outputFile string) error {
// 	// Start inotify to watch for changes
// 	fmt.Println("Starting inotify for watching config map update")

// 	_, err := os.Create(outputFile)
// 	if err != nil {
// 		log.Fatalf("Error creating output file: %v\n", err)
// 	}

// 	// Define the command to start inotify
// 	inotifyCommand := exec.Command(
// 		"inotifywait",
// 		"/etc/config/settings",
// 		"--daemon",
// 		"--recursive",
// 		"--outfile", outputFile,
// 		"--event", "create,delete",
// 		"--format", "%e : %T",
// 		"--timefmt", "+%s",
// 	)

// 	// Start the inotify process
// 	err = inotifyCommand.Start()
// 	if err != nil {
// 		log.Fatalf("Error starting inotify process: %v\n", err)
// 	}

// 	return nil
// }

func hasConfigChanged(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Println("Error getting file info:", err)
			os.Exit(1)
		}

		return fileInfo.Size() > 0
	}
	return false
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\ntargetallocator is running."

	if hasConfigChanged("/opt/inotifyoutput-ta-config.txt") {
		status = http.StatusServiceUnavailable
		message += "\ninotifyoutput.txt has been updated - targetallocator-config changed"
	}

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	if status != http.StatusOK {
		fmt.Printf(message)
	}
}

func startCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())
	// // Print the environment variables being passed into the cmd
	// fmt.Println("Environment variables being passed into the command:")
	// for _, v := range cmd.Env {
	// 	fmt.Println(v)
	// }
	// Create pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe: %v\n", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe: %v\n", err)
		return
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting command: %v\n", err)
		return
	}

	// Create goroutines to capture and print stdout and stderr
	go func() {
		stdoutBytes, _ := ioutil.ReadAll(stdout)
		fmt.Print(string(stdoutBytes))
	}()

	go func() {
		stderrBytes, _ := ioutil.ReadAll(stderr)
		fmt.Print(string(stderrBytes))
	}()
}

func main() {
	// Starting inotify to watch for config file changes
	fmt.Println("Starting inotify for watching targetallocator config update")

	// taConfigFilePath := "/conf/targetallocator.yaml"
	taConfigFilePath := "/conf"

	// Create an output file for inotify events
	//outputFile := "/opt/inotifyoutput-ta-config.txt"
	_, err := os.Create(inotify_config_output_file)
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
	}

	// Define the command to start inotify
	inotifyCommand := exec.Command(
		"inotifywait",
		taConfigFilePath,
		"--daemon",
		"--outfile", inotify_config_output_file,
		"--event", "ATTRIB",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommand.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process: %v\n", err)
	}

	// Start targetallocator
	//startCommand("/opt/targetallocator", "--enable-prometheus-cr-watcher")

	http.HandleFunc("/health", healthHandler)
	http.ListenAndServe(":8081", nil)
	// go forever()
	// select {} // block forever
}

// func forever() {
// 	for {
// 		fmt.Printf("%v+\n", time.Now())
// 		time.Sleep(time.Second)
// 	}
// }
