package main

import (
	"fmt"
	"os"

	"go.goms.io/aks/configmap-watcher/cmd"
)

func main() {
	var cli cmd.KubeClient = &cmd.Kubectl{}
	rootCmd := cmd.NewKubeCommand(cli)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
