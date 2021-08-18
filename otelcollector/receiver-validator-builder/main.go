package main

import (
	"fmt"
	"log"
)

func main() {
	receiver, err := components()
	if err != nil {
		log.Fatalf("failed to build receiver: %v", err)
	}
	// info := component.BuildInfo{
	// 	Command:     "custom-receiver-validator",
	// 	Description: "Custom Receiver validator",
	// 	Version:     "1.0.0",
	// }
	fmt.Printf("Receiver: %v", receiver)

}
