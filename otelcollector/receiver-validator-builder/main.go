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

}
