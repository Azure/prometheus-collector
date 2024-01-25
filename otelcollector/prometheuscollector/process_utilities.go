package main

import (
    "fmt"
	"io"
    "os"
    "strings"
    "os/exec"
    "io/ioutil"
)

func isProcessRunning(processName string) bool {
    // List all processes in the current process group
    pid := os.Getpid()
    processes, err := os.ReadDir("/proc")
    if err != nil {
        fmt.Println("Error:", err)
        return false
    }

    for _, processDir := range processes {
        if processDir.IsDir() {
            processID := processDir.Name()
            _, err := os.Stat("/proc/" + processID + "/cmdline")
            if err == nil {
                cmdline, err := os.ReadFile("/proc/" + processID + "/cmdline")
                if err == nil {
                    if strings.Contains(string(cmdline), processName) {
                        // Skip the current process (this program)
                        if processID != fmt.Sprintf("%d", pid) {
                            return true
                        }
                    }
                }
            }
        }
    }

    return false
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

func startCommandAndWait(command string, args ...string) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	// cmd.Env = append(os.Environ())
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

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		fmt.Printf("Error waiting for command: %v\n", err)
	}
}

func copyOutput(src io.Reader, dest io.Writer, file *os.File) {
	// Create a multi-writer to write to both the file and os.Stdout/os.Stderr
	multiWriter := io.MultiWriter(dest, file)

	_, err := io.Copy(multiWriter, src)
	if err != nil {
		fmt.Printf("Error copying output: %v\n", err)
	}
}
func startMetricsExtensionWithConfigOverrides(configOverrides string) {
	cmd := exec.Command("/usr/sbin/MetricsExtension", "-Logger", "Console", "-LogLevel", "Error", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", "/usr/sbin/me.config")
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

	// Goroutines to copy stdout and stderr to parent process
	go copyOutput(stdout, os.Stdout, "metricsextension_stdout.log")
	go copyOutput(stderr, os.Stderr, "metricsextension_sterr.log")

    // Start the command
    err = cmd.Start()
    if err != nil {
        fmt.Printf("Error starting MetricsExtension: %v\n", err)
        return
    }
}
