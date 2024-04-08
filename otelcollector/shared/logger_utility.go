package main

import (
	"fmt"
)

const (
	Color_Off = "\033[0m"
	Red       = "\033[0;31m"
	Green     = "\033[0;32m"
	Yellow    = "\033[0;33m"
	Cyan      = "\033[0;36m"
)

// Echo text in red
func echoError(msg string) {
	fmt.Printf("%s%s%s\n", Red, msg, Color_Off)
}

// Echo text in yellow
func echoWarning(msg string) {
	fmt.Printf("%s%s%s\n", Yellow, msg, Color_Off)
}

// Echo variable name in Cyan and value in regular color
func echoVar(name, value string) {
	fmt.Printf("%s%s%s=%s\n", Cyan, name, Color_Off, value)
}
