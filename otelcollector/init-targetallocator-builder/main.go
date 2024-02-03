package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)


func main() {
	// Waiting until config file is created by the sidecar
	while not os.path.exists("/conf/targetallocator.yaml"):
    time.sleep(1)

	fmt.Println("Config file created at /conf/targetallocator.yaml")

	os.Exit(0)
}


