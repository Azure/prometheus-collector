package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// SetEnvAndSourceBashrc sets a key-value pair as an environment variable in the .bashrc file
// and sources the file to apply changes immediately.
func SetEnvAndSourceBashrc(key, value string) error {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user's home directory: %v", err)
	}

	// Construct the path to .bashrc
	bashrcPath := filepath.Join(homeDir, ".bashrc")

	// Check if .bashrc exists, if not, create it
	if _, err := os.Stat(bashrcPath); os.IsNotExist(err) {
		file, err := os.Create(bashrcPath)
		if err != nil {
			return fmt.Errorf("failed to create .bashrc file: %v", err)
		}
		defer file.Close()
	}

	// Open the .bashrc file for appending
	file, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .bashrc file: %v", err)
	}
	defer file.Close()

	// Write the export statement to the .bashrc file
	_, err = fmt.Fprintf(file, "export %s=%s\n", key, value)
	if err != nil {
		return fmt.Errorf("failed to write to .bashrc file: %v", err)
	}

	// Source the .bashrc file
	cmd := exec.Command("bash", "-c", "source "+bashrcPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to source .bashrc: %v", err)
	}

	// Set the environment variable
	err = os.Setenv(key, value)
	if err != nil {
		return fmt.Errorf("failed to set environment variable: %v", err)
	}

	return nil
}

func startCommand(command string, args ...string) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())

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
	cmd.Env = append(os.Environ())
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

func copyOutputMulti(src io.Reader, dest io.Writer, file *os.File) {
	// Create a multi-writer to write to both the file and os.Stdout/os.Stderr
	multiWriter := io.MultiWriter(dest, file)

	_, err := io.Copy(multiWriter, src)
	if err != nil {
		fmt.Printf("Error copying output: %v\n", err)
	}
}

func copyOutputPipe(src io.Reader, dest io.Writer) {
	_, err := io.Copy(dest, src)

	if err != nil {
		fmt.Printf("Error copying output: %v\n", err)
	}
}

func copyOutputFile(src io.Reader, file *os.File) {
	_, err := io.Copy(file, src)

	if err != nil {
		fmt.Printf("Error copying output: %v\n", err)
	}
}

func startMetricsExtensionWithConfigOverrides(configOverrides string) {
	cmd := exec.Command("/usr/sbin/MetricsExtension", "-Logger", "Console", "-LogLevel", "Error", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", "/usr/sbin/me.config")

	// Create a file to store the stdoutput
	// metricsextension_stdout_file, err := os.Create("metricsextension_stdout.log")
	// if err != nil {
	// 	fmt.Printf("Error creating output file for metrics extension: %v\n", err)
	// 	return
	// }

	// // Create a file to store the stderr
	// metricsextension_stderr_file, err := os.Create("metricsextension_stderr.log")
	// if err != nil {
	// 	fmt.Printf("Error creating output file for metrics extension: %v\n", err)
	// 	return
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

	// Goroutines to copy stdout and stderr to parent process
	// Copy output to only stdout & stderr
	go copyOutputPipe(stdout, os.Stdout)
	go copyOutputPipe(stderr, os.Stderr)

	// Copy output to both stdout & stderr and file
	// go copyOutputMulti(stdout, os.Stdout, metricsextension_stdout_file)
	// go copyOutputMulti(stderr, os.Stderr, metricsextension_stderr_file)

	// Copy output only to file
	// go copyOutputFile(stdout, metricsextension_stdout_file)
	// go copyOutputFile(stderr, metricsextension_stderr_file)

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MetricsExtension: %v\n", err)
		return
	}
}

func startMdsd() {
	cmd := exec.Command("/usr/sbin/mdsd", "-a", "-A", "-D")
	// // Create a file to store the stdoutput
	// mdsd_stdout_file, err := os.Create("mdsd_stdout.log")
	// if err != nil {
	// 	fmt.Printf("Error creating output file for mdsd: %v\n", err)
	// 	return
	// }

	// // Create a file to store the stderr
	// mdsd_stderr_file, err := os.Create("mdsd_stderr.log")
	// if err != nil {
	// 	fmt.Printf("Error creating output file for mdsd: %v\n", err)
	// 	return
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

	// Goroutines to copy stdout and stderr to parent process
	// Copy output to only stdout and stderr
	go copyOutputPipe(stdout, os.Stdout)
	go copyOutputPipe(stderr, os.Stderr)

	// Copy output to both stdout and file
	// go copyOutputMulti(stdout, os.Stdout, mdsd_stdout_file)
	// go copyOutputMulti(stderr, os.Stderr, mdsd_stderr_file)

	// Copy output only to file
	// go copyOutputFile(stdout, mdsd_stdout_file)
	// go copyOutputFile(stderr, mdsd_stderr_file)

	// Start the command
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting mdsd: %v\n", err)
		return
	}
}
