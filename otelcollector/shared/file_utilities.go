package shared

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

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
	fmt.Printf("%s=\"%s\"\n", name, value)
}

func ExistsAndNotEmpty(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		fmt.Println("ExistsAndNotEmpty: file:", filename, "doesn't exist")
		return false
	}
	if err != nil {
		fmt.Println("ExistsAndNotEmpty: path:", filename, ":error:", err)
		return false
	}
	if info.Size() == 0 {
		fmt.Println("ExistsAndNotEmpty: file size is 0 for:", filename)
		return false
	}
	return true
}

func ReadAndTrim(filename string) (string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	trimmedContent := strings.TrimSpace(string(content))
	return trimmedContent, nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func CopyFile(sourcePath, destinationPath string) error {
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

// FileExists checks if a file exists and is not a directory before we
// try using it to prevent further errors.
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func SetEnvVarsFromFile(filename string) error {
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

		SetEnvAndSourceBashrcOrPowershell(key, value, false)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func Inotify(outputFile string, location1 string, location2 string) error {
	// Start inotify to watch for changes
	fmt.Println("Starting inotify for watching config map update")

	_, err := os.Create(outputFile)
	if err != nil {
		log.Fatalf("Error creating output file: %v\n", err)
		fmt.Println("Error creating inotify output file:", err)
	}

	// Define the command to start inotify
	inotifyCommand := exec.Command(
		"inotifywait",
		location1,
		location2,
		"--daemon",
		"--recursive",
		"--outfile", outputFile,
		"--event", "create,delete,modify",
		"--format", "%e : %T",
		"--timefmt", "+%s",
	)

	// Start the inotify process
	err = inotifyCommand.Start()
	if err != nil {
		log.Fatalf("Error starting inotify process: %v\n", err)
		fmt.Println("Error starting inotify process:", err)
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

func AddLineToBashrc(line string) error {
	// Get the home directory of the current user
	currentUser, err := user.Current()
	if err != nil {
		return err
	}
	homeDir := currentUser.HomeDir

	// Find the .bashrc file path
	bashrcPath := filepath.Join(homeDir, ".bashrc")

	// Check if the line already exists in .bashrc
	if exists, err := lineExistsInFile(bashrcPath, line); err != nil {
		return err
	} else if exists {
		return nil // Line already exists, no need to add it again
	}

	// Open .bashrc file in append mode
	file, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Add the line to .bashrc
	_, err = file.WriteString(line + "\n")
	if err != nil {
		return err
	}

	return nil
}

// Function to check if a line exists in a file
func lineExistsInFile(filePath, line string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == line {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func ModifyConfigFile(configFile string, pid int, placeholder string) error {
	// Read the contents of the config file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	// Replace placeholder with the actual PID
	replacedContent := bytes.ReplaceAll(content, []byte(placeholder), []byte(fmt.Sprintf("%d", pid)))

	// Write the modified content back to the config file
	err = os.WriteFile(configFile, replacedContent, 0644)
	if err != nil {
		return fmt.Errorf("error writing to config file: %v", err)
	}

	return nil
}
