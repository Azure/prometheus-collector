package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// env variable which has ResourceName for NON-AKS
const ResourceNameEnv = "ACS_RESOURCE_NAME"

// env variable which has container run time name
const ContainerRuntimeEnv = "CONTAINER_RUNTIME"

var (
	// PluginConfiguration the plugins configuration
	PluginConfiguration map[string]string
	// Computer (Hostname) when ingesting into ContainerLog table
	Computer string
	// ResourceID for resource-centric log analytics data
	ResourceID string
	// Resource-centric flag (will be true if we determine if above RseourceID is non-empty - default is false)
	ResourceCentric bool
	//ResourceName
	ResourceName string
)

var (
	// FLBLogger stream
	FLBLogger = createLogger()
	// Log wrapper function
	Log = FLBLogger.Printf
)

var (
	dockerCimprovVersion = "9.0.0.0"
	agentName            = "ContainerAgent"
	userAgent            = ""
)

func createLogger() *log.Logger {
	var logfile *os.File
	var logPath = "/opt/fluent-bit/fluent-bit-out-appinsights-runtime.log"

	if _, err := os.Stat(logPath); err == nil {
		fmt.Printf("File Exists. Opening file in append mode...\n")
		logfile, err = os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			SendException(err.Error())
			fmt.Printf(err.Error())
		}
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("File Doesnt Exist. Creating file...\n")
		logfile, err = os.Create(logPath)
		if err != nil {
			SendException(err.Error())
			fmt.Printf(err.Error())
		}
	}

	logger := log.New(logfile, "", 0)

	logger.SetOutput(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10, //megabytes
		MaxBackups: 1,
		MaxAge:     28,   //days
		Compress:   true, // false by default
	})

	logger.SetFlags(log.Ltime | log.Lshortfile | log.LstdFlags)
	return logger
}

// InitializePlugin reads and populates plugin configuration
func InitializePlugin(agentVersion string) {

	go func() {
		isTest := os.Getenv("ISTEST")
		if strings.Compare(strings.ToLower(strings.TrimSpace(isTest)), "true") == 0 {
			e1 := http.ListenAndServe("localhost:6060", nil)
			if e1 != nil {
				Log("HTTP Listen Error: %s \n", e1.Error())
			}
		}
	}()

	os_type := os.Getenv("OS_TYPE")
	Log("OS_TYPE=%s", os_type)

	ResourceID = os.Getenv(envCluster)
	Computer = os.Getenv(envComputerName)

	if len(ResourceID) > 0 {
		//AKS Scenario
		ResourceCentric = true
		splitted := strings.Split(ResourceID, "/")
		ResourceName = splitted[len(splitted)-1]
		Log("ResourceCentric: True")
		Log("ResourceID=%s", ResourceID)
		Log("ResourceName=%s", ResourceID)
	}
	if ResourceCentric == false {
		//AKS-Engine/hybrid scenario
		ResourceName = os.Getenv(ResourceNameEnv)
		ResourceID = ResourceName
		Log("ResourceCentric: False")
		Log("ResourceID=%s", ResourceID)
		Log("ResourceName=%s", ResourceName)
	}

	// set useragent to be used by ingestion
	dockerCimprovVersionEnv := strings.TrimSpace(os.Getenv("DOCKER_CIMPROV_VERSION"))
	if len(dockerCimprovVersionEnv) > 0 {
		dockerCimprovVersion = dockerCimprovVersionEnv
	}

	userAgent = fmt.Sprintf("%s/%s", agentName, dockerCimprovVersion)
	Log("Usage-Agent = %s \n", userAgent)
	Log("Computer == %s \n", Computer)

	ret, err := InitializeTelemetryClient(agentVersion)
	if ret != 0 || err != nil {
		message := fmt.Sprintf("Error During Telemetry Initialization :%s", err.Error())
		fmt.Printf(message)
		Log(message)
	}
}
