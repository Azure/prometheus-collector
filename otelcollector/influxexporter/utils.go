package influxexporter

import (
	"net"
	"time"
	"fmt"
	"errors"
	"os"
	"log"
	"strings"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	MEInfluxUnixSocketClient net.Conn
	FileLogger = createFileLogger()
	Log = FileLogger.Printf
)

//ME client to write influx data
func CreateMEClient() {
	if MEInfluxUnixSocketClient != nil {
		MEInfluxUnixSocketClient.Close()
		MEInfluxUnixSocketClient = nil
	}
	conn, err := net.DialTimeout("tcp",
		"0.0.0.0:8089", 10*time.Second)
	if err != nil {
		Log("Error::ME::Unable to open ME influx TCP socket connection %s", err.Error())
	} else {
		Log("Successfully created ME influx TCP socket connection")
		MEInfluxUnixSocketClient = conn
	}
}

func Write2ME(messages []byte) (numBytesWritten int, e error) {
	if MEInfluxUnixSocketClient == nil {
		Log("ME connection does not exist. Creating...")
		CreateMEClient()
	}
	if MEInfluxUnixSocketClient != nil {
		MEInfluxUnixSocketClient.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if messages != nil && len(messages) > 0 {
			bytesWritten, e := MEInfluxUnixSocketClient.Write(messages)
			if e != nil { MEInfluxUnixSocketClient = nil}
			return bytesWritten, e
		}
	} 
	return 0, errors.New("Error opening TCP connection to ME")
}

func createFileLogger() *log.Logger {
	var logfile *os.File

	osType := os.Getenv("OS_TYPE")

	var logPath string

	if strings.Compare(strings.ToLower(osType), "windows") != 0 {
		logPath = "/opt/microsoft/otelcollector/influx-exporter.log"
	} else {
		logPath = "/etc/omsagentwindows/influx-exporter.log"
	}

	if _, err := os.Stat(logPath); err == nil {
		fmt.Printf("Log file Exists. Opening file in append mode...\n")
		logfile, err = os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			//SendException(err.Error())
			fmt.Printf(err.Error())
		}
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		fmt.Printf("Log file doesn't Exist. Creating file...\n")
		logfile, err = os.Create(logPath)
		if err != nil {
			//SendException(err.Error())
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