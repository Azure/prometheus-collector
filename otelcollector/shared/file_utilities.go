package shared

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getPath(file string) string {
	if os.Getenv("GO_ENV") == "test" {
		dir := filepath.Join(".", file)
		return dir
	}

	return file
}

func PrintMdsdVersion() {
	cmd := exec.Command("mdsd", "--version")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting MDSD version: %v\n", err)
		return
	}

	FmtVar("MDSD_VERSION", string(output))
}

func ReadVersionFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func FmtVar(name, value string) {
	fmt.Printf("%s=\"%s\"\n", name, strings.TrimRight(value, "\n\r"))
}

func existsAndNotEmpty(filename string) bool {
	info, err := os.Stat(getPath(filename))
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
	content, err := os.ReadFile(getPath(filename))
	if err != nil {
		return "", err
	}
	trimmedContent := strings.TrimSpace(string(content))
	return trimmedContent, nil
}

func exists(path string) bool {
	_, err := os.Stat(getPath(path))
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func copyFile(sourcePath, destinationPath string) error {
	sourceFile, err := os.Open(getPath(sourcePath))
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(getPath(destinationPath))
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

func setEnvVarsFromFile(filePath string) error {
	// Check if the file exists
	filename := getPath(filePath)
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

		// Set the environment variable
		err := os.Setenv(key, value)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func MonitorInotify(outputFile string) error {
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

func HasConfigChanged(filePath string) bool {
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

func WriteTerminationLog(message string) {
	if err := os.WriteFile("/dev/termination-log", []byte(message), fs.FileMode(0644)); err != nil {
		log.Printf("Error writing to termination log: %v", err)
	}
}
