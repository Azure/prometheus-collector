package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
)

func printMdsdVersion() {
	cmd := exec.Command("mdsd", "--version")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting MDSD version: %v\n", err)
		return
	}
	fmtVar("MDSD_VERSION", string(output))
}

func readVersionFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func fmtVar(name, value string) {
	fmt.Printf("%s=\"%s\"\n", name, strings.TrimRight(value, "\n\r"))
}

func existsAndNotEmpty(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return false
	}
	if info.Size() == 0 {
		return false
	}
	return true
}

func readAndTrim(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	trimmedContent := strings.TrimSpace(string(content))
	return trimmedContent, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func copyFile(sourcePath, destinationPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}

// setEnvAndSourceBashrc sets a key-value pair as an environment variable in the .bashrc file
// and sources the file to apply changes immediately.
func setEnvAndSourceBashrc(key, value string) error {
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

func setEnvVarsFromFile(filename string) error {
	// Check if the file exists
	_, e := os.Stat(filename)
	if os.IsNotExist(e) {
		return fmt.Errorf("File does not exist: %s", filename)
	}
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			fmt.Printf("Skipping invalid line: %s\n", line)
			continue
		}

		key := parts[0]
		value := parts[1]

		setEnvAndSourceBashrc(key, value)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func monitorInotify(outputFile string) error {
	// Start inotify to watch for changes
	fmt.Println("Starting inotify for watching config map update")

	_, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
	}

	// Define the command to start inotify
	inotifyCommand := exec.Command(
		"inotifywait",
		"/etc/config/settings",
		"--daemon",
		"--recursive",
		"--outfile", outputFile,
		"--event", "create,delete",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommand.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process: %v\n", err)
	}

	return nil
}

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

func writeTerminationLog(message string) {
	if err := os.WriteFile("/dev/termination-log", []byte(message), fs.FileMode(0644)); err != nil {
		log.Printf("Error writing to termination log: %v", err)
	}
}
