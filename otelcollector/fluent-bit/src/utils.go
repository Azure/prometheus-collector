package main

import (
	"errors"
	"io/ioutil"
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
