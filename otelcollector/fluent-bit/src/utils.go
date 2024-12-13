package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
)

// ToString converts an interface into a string
func ToString(s interface{}) string {
	switch t := s.(type) {
	case []byte:
		// prevent encoding to base64
		return string(t)
	default:
		return ""
	}
}

func ReadFileContents(fullPathToFileName string) (string, error) {
	fullPathToFileName = strings.TrimSpace(fullPathToFileName)
	if len(fullPathToFileName) == 0 {
		return "", errors.New("ReadFileContents::filename is empty")
	}
	content, err := ioutil.ReadFile(fullPathToFileName) //no need to close
	if err != nil {
		return "", errors.New("ReadFileContents::Unable to open file " + fullPathToFileName)
	} else {
		return strings.TrimSpace(string(content)), nil
	}
}

// From telegraf codebase
func findPIDFromExe(process string, os string) ([]int32, error) {
	var command *exec.Cmd
	if os == "windows" {
		command = exec.Command("powershell", "-Command", fmt.Sprintf("Get-Process -Name %s | Select-Object -Expand Id", process))
	} else {
		command = exec.Command("pgrep", process)
	}
	buf, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("error running %w", err)
	}
	out := string(buf)

	fields := strings.Fields(out)

	fmt.Printf("fields: %v\n", fields)
	pids := make([]int32, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.ParseInt(field, 10, 32)
		if err != nil {
			return nil, err
		}
		pids = append(pids, int32(pid))
	}

	fmt.Printf("pids: %v\n", pids)

	return pids, nil
}
