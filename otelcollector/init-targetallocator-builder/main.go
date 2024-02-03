package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	// Waiting until config file is created by the sidecar
	if _, err := os.Stat("/conf/targetallocator.yaml"); err == nil {
		fmt.Println("Config file created at /conf/targetallocator.yaml")
		os.Exit(0)

	} else {
		time.Sleep(1 * time.Second)
	}
}
