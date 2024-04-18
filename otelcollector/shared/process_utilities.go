package shared

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func IsProcessRunning(processName string) bool {
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

func StartCommandWithOutputFile(command string, args []string, outputFile string) error {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())

	// Create file to write stdout and stderr
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}

	// Create pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	// Create a wait group to wait for goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// Create goroutines to continuously read and write stdout and stderr
	go func() {
		defer wg.Done()
		if _, err := io.Copy(file, stdout); err != nil {
			fmt.Printf("Error copying stdout to file: %v\n", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := io.Copy(file, stderr); err != nil {
			fmt.Printf("Error copying stderr to file: %v\n", err)
		}
	}()

	// Wait for both goroutines to finish before closing the file
	go func() {
		wg.Wait()
		file.Close()
	}()

	return nil
}

func StartCommand(command string, args ...string) {
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

func StartCommandAndWait(command string, args ...string) {
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

func StartMetricsExtensionForOverlay(meConfigFile string) {
	cmd := exec.Command("/usr/sbin/MetricsExtension", "-Logger", "File", "-LogLevel", "Info", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", meConfigFile)
	// Start the command
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MetricsExtension: %v\n", err)
		return
	}
}

func StartMetricsExtensionWithConfigOverrides(configOverrides string) {
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

func StartMdsdForOverlay() {
	mdsdLog := os.Getenv("MDSD_LOG")
	if mdsdLog == "" {
		fmt.Println("MDSD_LOG environment variable is not set")
		return
	}

	cmd := exec.Command("/usr/sbin/mdsd", "-a", "-A", "-e", mdsdLog+"/mdsd.err", "-w", mdsdLog+"/mdsd.warn", "-o", mdsdLog+"/mdsd.info", "-q", mdsdLog+"/mdsd.qos")
	// Redirect stderr to /dev/null
	cmd.Stderr = nil
	// Start the command
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting mdsd: %v\n", err)
		return
	}
}

func StartMdsd() {
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
