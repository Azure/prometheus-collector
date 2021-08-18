package main

import (
	"fmt"
)

func main() {
	receiver := components()
	// info := component.BuildInfo{
	// 	Command:     "custom-receiver-validator",
	// 	Description: "Custom Receiver validator",
	// 	Version:     "1.0.0",
	// }
	// fmt.Printf("Receiver: %v", receiver)
	fmt.Printf("Receiver: %+v\n", receiver)
	cfg := receiver.CreateDefaultConfig()
	fmt.Printf("Config: %+v\n", cfg)

	err := cfg.Validate()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}
