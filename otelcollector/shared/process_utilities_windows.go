package shared

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// IsProcessRunning checks if a process with the given name is running on the system
func IsProcessRunning(processName string) bool {
	osType := os.Getenv("OS_TYPE")

	switch osType {
	case "linux":
		return isProcessRunningLinux(processName)
	case "windows":
		return isProcessRunningWindows(processName)
	default:
		fmt.Println("Unsupported OS_TYPE:", osType)
		return false
	}
}

// Linux implementation using the /proc directory
func isProcessRunningLinux(processName string) bool {
	pid := os.Getpid()
	dir, err := os.Open("/proc")
	if err != nil {
		fmt.Println("Error opening /proc:", err)
		return false
	}
	defer dir.Close()

	procs, err := dir.Readdirnames(0)
	if err != nil {
		fmt.Println("Error reading /proc:", err)
		return false
	}

	for _, proc := range procs {
		if _, err := os.Stat("/proc/" + proc + "/cmdline"); err == nil {
			cmdline, err := os.ReadFile("/proc/" + proc + "/cmdline")
			if err == nil && strings.Contains(string(cmdline), processName) {
				if proc != fmt.Sprintf("%d", pid) {
					return true
				}
			}
		}
	}
	return false
}

type ProcessEntry32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [260]uint16 // Process name
}

// Windows implementation using syscalls
func isProcessRunningWindows(processName string) bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procSnapshot := kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcessFirst := kernel32.NewProc("Process32FirstW")
	procProcessNext := kernel32.NewProc("Process32NextW")
	handle, _, _ := procSnapshot.Call(2, 0) // TH32CS_SNAPPROCESS
	if handle == 0 {
		fmt.Println("Error getting snapshot of processes")
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(handle))
	var entry ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	// Get the first process
	ret, _, _ := procProcessFirst.Call(handle, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		// Convert UTF-16 file name to string
		exeFile := syscall.UTF16ToString(entry.ExeFile[:])

		// Case-insensitive comparison
		if strings.EqualFold(exeFile, processName) {
			return true
		}
		// Move to the next process
		ret, _, _ = procProcessNext.Call(handle, uintptr(unsafe.Pointer(&entry)))
	}
	return false
}

// SetEnvAndSourceBashrcOrPowershell sets a key-value pair as an environment variable.
// If OS_TYPE is 'linux', it sets the variable in the .bashrc file and sources it.
// If OS_TYPE is 'windows', it sets the variable in the system environment.
func SetEnvAndSourceBashrcOrPowershell(key, value string, echo bool) error {
	// Get the OS_TYPE from environment variables
	osType := os.Getenv("OS_TYPE")

	if osType == "linux" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user's home directory: %v", err)
		}

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

		_, err = fmt.Fprintf(file, "export %s=%s\n", key, value)
		if err != nil {
			return fmt.Errorf("failed to write to .bashrc file: %v", err)
		}

		// Source the .bashrc file
		cmd := exec.Command("bash", "-c", "source "+bashrcPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to source .bashrc: %v", err)
		}

	} else if osType == "windows" {
		// On Windows, set the environment variable for the machine (persistent across sessions)
		cmd := exec.Command("setx", key, value, "/M") // "/M" flag sets the variable for the machine
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set environment variable on Windows (Machine scope): %v", err)
		}

		// Set the environment variable for the current process
		err := os.Setenv(key, value)
		if err != nil {
			return fmt.Errorf("failed to set environment variable for current session: %v", err)
		}
	} else {
		return fmt.Errorf("unsupported OS_TYPE: %s", osType)
	}

	// Conditionally call EchoVar
	if echo {
		EchoVar(key, value)
	}

	return nil
}

func StartCommandWithOutputFile(command string, args []string, outputFile string) (int, error) {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())

	// Create file to write stdout and stderr
	file, err := os.Create(outputFile)
	if err != nil {
		return 0, fmt.Errorf("error creating output file: %v", err)
	}

	// Create pipes to capture stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("error creating stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("error starting command: %v", err)
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

	// Get the PID of the started process
	process_pid := cmd.Process.Pid

	return process_pid, nil
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
		stdoutBytes, _ := io.ReadAll(io.Reader(stdout))
		fmt.Print(string(stdoutBytes))
	}()

	go func() {
		stderrBytes, _ := io.ReadAll(io.Reader(stderr))
		fmt.Print(string(stderrBytes))
	}()
}

func StartCommandAndWait(command string, args ...string) error {
	cmd := exec.Command(command, args...)

	// Set environment variables from os.Environ()
	cmd.Env = append(os.Environ())

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
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	// Create goroutines to capture and print stdout and stderr
	go func() {
		stdoutBytes, _ := io.ReadAll(io.Reader(stdout))
		fmt.Print(string(stdoutBytes))
	}()

	go func() {
		stderrBytes, _ := io.ReadAll(io.Reader(stderr))
		fmt.Print(string(stderrBytes))
	}()

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("error waiting for command: %v", err)
	}

	return nil
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

// StartMetricsExtensionForOverlay starts the MetricsExtension process based on the OS
func StartMetricsExtensionForOverlay(meConfigFile string) (int, error) {
	osType := os.Getenv("OS_TYPE")
	var cmd *exec.Cmd

	switch osType {
	case "linux":
		cmd = exec.Command("/usr/sbin/MetricsExtension", "-Logger", "File", "-LogLevel", "Info", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", meConfigFile)

	case "windows":
		cmd = exec.Command("C:\\opt\\metricextension\\MetricsExtension\\MetricsExtension.Native.exe", "-Logger", "File", "-LogLevel", "Info", "-DataDirectory", ".\\", "-Input", "otlp_grpc_prom", "-MonitoringAccount", os.Getenv("AZMON_DEFAULT_METRIC_ACCOUNT_NAME"), "-ConfigOverridesFilePath", meConfigFile)
	}

	cmd.Env = append(os.Environ())

	err := cmd.Start()
	if err != nil {
		return 0, fmt.Errorf("error starting MetricsExtension: %v", err)
	}

	return cmd.Process.Pid, nil
}

func StartMetricsExtensionWithConfigOverridesForUnderlay(configOverrides string) {
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

func StartMA() {
	osType := os.Getenv("OS_TYPE")
	var cmd *exec.Cmd

	switch osType {
	case "linux":
		fmt.Println("Should never reach here")

	case "windows":
		cmd = exec.Command("C:\\opt\\genevamonitoringagent\\genevamonitoringagent\\Monitoring\\Agent\\MonAgentLauncher.exe", "-useenv")
		// On Windows, stderr redirection is not needed as `cmd.Start()` handles it internally
	}

	// Start the command
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting mdsd/MonAgentLauncher: %v\n", err)
		return
	}

	fmt.Printf("%s process started successfully.\n", cmd.Path)
}

// StartMdsdForOverlay starts the mdsd process based on the OS
func StartMdsdForOverlay() {
	osType := os.Getenv("OS_TYPE")
	var cmd *exec.Cmd

	switch osType {
	case "linux":
		mdsdLog := os.Getenv("MDSD_LOG")
		if mdsdLog == "" {
			fmt.Println("MDSD_LOG environment variable is not set")
			return
		}
		cmd = exec.Command("/usr/sbin/mdsd", "-a", "-A", "-e", mdsdLog+"/mdsd.err", "-w", mdsdLog+"/mdsd.warn", "-o", mdsdLog+"/mdsd.info", "-q", mdsdLog+"/mdsd.qos")
		// Redirect stderr to /dev/null
		cmd.Stderr = nil

	case "windows":
		cmd = exec.Command("C:\\opt\\genevamonitoringagent\\genevamonitoringagent\\Monitoring\\Agent\\MonAgentLauncher.exe", "-useenv")
		// On Windows, stderr redirection is not needed as `cmd.Start()` handles it internally
	}

	// Start the command
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting mdsd/MonAgentLauncher: %v\n", err)
		return
	}

	fmt.Printf("%s process started successfully.\n", cmd.Path)
}

func StartMdsdForUnderlay() {
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

func StartCronDaemon() {
	cmd := exec.Command("/usr/sbin/crond", "-n", "-s")
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
}

func WaitForTokenAdapter(ccpMetricsEnabled string) {
	tokenAdapterWaitSecs := 60
	if ccpMetricsEnabled == "true" {
		tokenAdapterWaitSecs = 20
	}
	waitedSecsSoFar := 1

	var resp *http.Response
	var err error

	client := &http.Client{Timeout: time.Duration(2) * time.Second}

	req, err := http.NewRequest("GET", "http://localhost:9999/healthz", nil)
	if err != nil {
		log.Printf("Unable to create http request for the healthz endpoint")
		return
	}
	for {
		if waitedSecsSoFar > tokenAdapterWaitSecs {
			if resp, err = client.Do(req); err != nil {
				log.Printf("giving up waiting for token adapter to become healthy after %d secs\n", waitedSecsSoFar)
				log.Printf("export tokenadapterUnhealthyAfterSecs=%d\n", waitedSecsSoFar)
				break
			}
		} else {
			log.Printf("checking health of token adapter after %d secs\n", waitedSecsSoFar)
			resp, err = client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				log.Printf("found token adapter to be healthy after %d secs\n", waitedSecsSoFar)
				log.Printf("export tokenadapterHealthyAfterSecs=%d\n", waitedSecsSoFar)
				break
			}
		}
		time.Sleep(1 * time.Second)
		waitedSecsSoFar++
	}

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
}

func StartFluentBit(fluentBitConfigFile string) {
	fmt.Println("Starting fluent-bit")
	if os.Getenv("OS_TYPE") == "linux" {
		if err := os.Mkdir("/opt/microsoft/fluent-bit", 0755); err != nil && !os.IsExist(err) {
			log.Fatalf("Error creating directory: %v\n", err)
		}

		logFile, err := os.Create("/opt/microsoft/fluent-bit/fluent-bit-out-appinsights-runtime.log")
		if err != nil {
			log.Fatalf("Error creating log file: %v\n", err)
		}
		defer logFile.Close()

		fluentBitCmd := exec.Command("fluent-bit", "-c", fluentBitConfigFile, "-e", "/opt/fluent-bit/bin/out_appinsights.so")
		fluentBitCmd.Stdout = os.Stdout
		fluentBitCmd.Stderr = os.Stderr
		if err := fluentBitCmd.Start(); err != nil {
			log.Fatalf("Error starting fluent-bit: %v\n", err)
		}
	} else {
		fluentBitCmd := exec.Command("C:\\opt\\fluent-bit\\bin\\fluent-bit.exe", "-c", "C:\\opt\\fluent-bit\\fluent-bit-windows.conf", "-e", "C:\\opt\\fluent-bit\\bin\\out_appinsights.so")
		fluentBitCmd.Stdout = os.Stdout
		fluentBitCmd.Stderr = os.Stderr

		if err := fluentBitCmd.Start(); err != nil {
			log.Fatalf("Error starting fluent-bit: %v\n", err)
		}

		// Run fluent-bit as a background process
		go func() {
			if err := fluentBitCmd.Wait(); err != nil {
				log.Printf("Fluent-bit exited with error: %v\n", err)
			}
		}()
	}
}
