package main

import (
    "fmt"
	"net/http"
    "os"
    "os/exec"
    "time"
    "io/ioutil"
    "strings"
)

func main() {
    // Run inotify as a daemon to track changes to the mounted configmap.
    runShellCommand("touch", "/opt/inotifyoutput.txt")
    runShellCommand("inotifywait", "/etc/config/settings", "--daemon", "--recursive", "--outfile", "/opt/inotifyoutput.txt", "--event", "create,delete", "--format", "'%e : %T'", "--timefmt", "'+%s'")


    controllerType := os.Getenv("controllerType")
	clusterOverride := os.Getenv("CLUSTER_OVERRIDE")

	var meConfigFile string

	if controllerType == "replicaset" {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me.config"
		}
	} else {
		if clusterOverride == "true" {
			meConfigFile = "/usr/sbin/me_ds_internal.config"
		} else {
			meConfigFile = "/usr/sbin/me_ds.config"
		}
	}
	fmt.Println("meConfigFile:", meConfigFile)

    if os.Getenv("MAC") == "true" {
		// Wait for addon-token-adapter to be healthy
		tokenAdapterWaitSecs := 60
		waitedSecsSoFar := 1

		for {
			if waitedSecsSoFar > tokenAdapterWaitSecs {
				_, err := http.Get("http://localhost:9999/healthz")
				if err != nil {
					fmt.Printf("giving up waiting for token adapter to become healthy after %d secs\n", waitedSecsSoFar)
					// Log telemetry about failure after waiting for waitedSecsSoFar and break
					fmt.Printf("export tokenadapterUnhealthyAfterSecs=%d\n", waitedSecsSoFar)
					break
				}
			} else {
				fmt.Printf("checking health of token adapter after %d secs\n", waitedSecsSoFar)
				resp, err := http.Get("http://localhost:9999/healthz")
				if err == nil && resp.StatusCode == http.StatusOK {
					fmt.Printf("found token adapter to be healthy after %d secs\n", waitedSecsSoFar)
					// Log telemetry about success after waiting for waitedSecsSoFar and break
					fmt.Printf("export tokenadapterHealthyAfterSecs=%d\n", waitedSecsSoFar)
					break
				}
			}

			time.Sleep(1 * time.Second)
			waitedSecsSoFar++
		}
    }

    meConfigFile := os.Getenv("meConfigFile")
	fluentBitConfigFile := os.Getenv("fluentBitConfigFile")
	mac := os.Getenv("MAC")
	cluster := os.Getenv("CLUSTER")
	aksRegion := os.Getenv("AKSREGION")
	mdsdLog := os.Getenv("MDSD_LOG")

	// Set environment variables
	os.Setenv("ME_CONFIG_FILE", meConfigFile)
	os.Setenv("FLUENT_BIT_CONFIG_FILE", fluentBitConfigFile)

	// Append environment variable assignments to ~/.bashrc
	appendEnvVarToBashrc("ME_CONFIG_FILE", meConfigFile)
	appendEnvVarToBashrc("FLUENT_BIT_CONFIG_FILE", fluentBitConfigFile)

	// Source ~/.bashrc
	sourceBashrc()

	if mac != "true" {
		if cluster == "" {
			fmt.Printf("CLUSTER is empty or not set. Using %s as CLUSTER\n", os.Getenv("NODE_NAME"))
			os.Setenv("customResourceId", os.Getenv("NODE_NAME"))
		} else {
			os.Setenv("customResourceId", cluster)
		}

		// Append customResourceId to ~/.bashrc
		appendEnvVarToBashrc("customResourceId", os.Getenv("customResourceId"))

		// Make a copy of the mounted akv directory
		copyAkvDirectory("/etc/config/settings/akv", "/opt/akv-copy")

		decodeLocation := "/opt/akv/decoded"
		ENCODEDFILES := listFiles("/etc/config/settings/akv")

		// Decode and set environment variables
		decodeAndSetEnvVars(decodeLocation, ENCODEDFILES)

		// Append AZMON_METRIC_ACCOUNTS_AKV_FILES to ~/.bashrc
		appendEnvVarToBashrc("AZMON_METRIC_ACCOUNTS_AKV_FILES", os.Getenv("AZMON_METRIC_ACCOUNTS_AKV_FILES"))

		fmt.Println("Starting metricsextension")

		// Start MetricsExtension with appropriate options
		startMetricsExtension()

	} else {
		os.Setenv("customResourceId", cluster)
		appendEnvVarToBashrc("customResourceId", os.Getenv("customResourceId"))

		trimmedRegion := strings.ReplaceAll(aksRegion, " ", "")
		trimmedRegion = strings.ToLower(trimmedRegion)
		os.Setenv("customRegion", trimmedRegion)
		appendEnvVarToBashrc("customRegion", os.Getenv("customRegion"))

		fmt.Println("Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests")
		time.Sleep(10 * time.Second)

		fmt.Println("Setting env variables from envmdsd file for MDSD")
		setEnvVarsFromEnvMdsdFile("/etc/mdsd.d/envmdsd")

		fmt.Println("Starting MDSD")
		startMdsd(mdsdLog)

		fmt.Print("MDSD_VERSION=")
		printMdsdVersion()

		fmt.Println("Waiting for 30s for MDSD to get the config and put them in place for ME")
		time.Sleep(30 * time.Second)

		fmt.Println("Reading me config file as a string for configOverrides parameter")
		meConfigString := readMeConfigFileAsString(meConfigFile)

		fmt.Println("Starting metricsextension")
		startMetricsExtensionWithConfigOverrides(meConfigString)
	}

    // Get ME version
	meVersion, err := readVersionFile("/opt/metricsextversion.txt")
	if err != nil {
		fmt.Printf("Error reading ME version file: %v\n", err)
	} else {
		fmtVar("ME_VERSION", meVersion)
	}

	// Get Ruby version
	rubyVersion, err := exec.Command("ruby", "--version").Output()
	if err != nil {
		fmt.Printf("Error getting Ruby version: %v\n", err)
	} else {
		fmtVar("RUBY_VERSION", string(rubyVersion))
	}

	// Get Golang version
	golangVersion, err := readVersionFile("/opt/goversion.txt")
	if err != nil {
		fmt.Printf("Error reading Golang version file: %v\n", err)
	} else {
		fmtVar("GOLANG_VERSION", golangVersion)
	}

	// Start otelcollector
	controllerType := os.Getenv("controllerType")
	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	azmonUseDefaultPrometheusConfig := os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG")

	var collectorConfig string

	if controllerType == "replicaset" && azmonOperatorEnabled == "true" {
		fmt.Println("Starting otelcollector in replicaset with Target allocator settings")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
	} else if azmonUseDefaultPrometheusConfig == "true" {
		// Commenting this out since config can be applied via CRD
		// fmt.Println("Starting otelcollector with only default scrape configs enabled")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config-default.yml"
	} else {
		fmt.Println("Starting otelcollector")
		collectorConfig = "/opt/microsoft/otelcollector/collector-config.yml"
	}

	cmd := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--config", collectorConfig)
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting otelcollector: %v\n", err)
	}

	otelCollectorVersion, err := exec.Command("/opt/microsoft/otelcollector/otelcollector", "--version").Output()
	if err != nil {
		fmt.Printf("Error getting otelcollector version: %v\n", err)
	} else {
		fmtVar("OTELCOLLECTOR_VERSION", string(otelCollectorVersion))
	}

	prometheusVersion, err := readVersionFile("/opt/microsoft/otelcollector/PROMETHEUS_VERSION")
	if err != nil {
		fmt.Printf("Error reading Prometheus version file: %v\n", err)
	} else {
		fmtVar("PROMETHEUS_VERSION", prometheusVersion)
	}

	if mac == "true" {
		// Start inotify to watch for changes
		fmt.Println("Starting inotify for watching mdsd config update")

		// Create an output file for inotify events
		outputFile := "/opt/inotifyoutput-mdsd-config.txt"
		_, err := os.Create(outputFile)
		if err != nil {
			log.Fatalf("Error creating output file: %v\n", err)
		}

		// Define the command to start inotify
		inotifyCommand := exec.Command(
			"inotifywait",
			"/etc/mdsd.d/config-cache/metricsextension/TokenConfig.json",
			"--daemon",
			"--outfile", outputFile,
			"--event", "ATTRIB",
			"--format", "%e : %T",
			"--timefmt", "+%s",
		)

		// Start the inotify process
		err = inotifyCommand.Start()
		if err != nil {
			log.Fatalf("Error starting inotify process: %v\n", err)
		}

		// Wait for the inotify process to finish (which won't happen as it's running as a daemon)
		err = inotifyCommand.Wait()
		if err != nil {
			log.Fatalf("Error waiting for inotify process: %v\n", err)
		}
	}

    // Expose a health endpoint for liveness probe
    http.HandleFunc("/health", healthHandler)
    http.ListenAndServe(":8080", nil)
}

func runShellCommand(command string, args ...string) {
    cmd := exec.Command(command, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    prometheuscollectorRunning := isProcessRunning("prometheuscollector")

    if prometheuscollectorRunning {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "prometheuscollector is running.")
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintln(w, "prometheuscollector is not running.")
    }
}

func isProcessRunning(processName string) bool {
    cmd := exec.Command("pgrep", processName)
    err := cmd.Run()
    return err == nil
}

func appendEnvVarToBashrc(name, value string) {
	bashrcFile := "/root/.bashrc"
	envVarAssignment := fmt.Sprintf("export %s=%s\n", name, value)
	err := ioutil.WriteFile(bashrcFile, []byte(envVarAssignment), os.ModeAppend)
	if err != nil {
		fmt.Printf("Error appending environment variable to ~/.bashrc: %v\n", err)
	}
}

func sourceBashrc() {
	cmd := exec.Command("bash", "-c", "source /root/.bashrc")
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error sourcing ~/.bashrc: %v\n", err)
	}
}

func copyAkvDirectory(sourceDir, destDir string) {
	cmd := exec.Command("cp", "-r", sourceDir, destDir)
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error copying AKV directory: %v\n", err)
	}
}

func listFiles(directory string) []string {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		fmt.Printf("Error listing files in directory: %v\n", err)
		return nil
	}

	var fileNames []string
	for _, file := range files {
		fileNames = append(fileNames, directory+"/"+file.Name())
	}
	return fileNames
}

func decodeAndSetEnvVars(decodeLocation string, encodedFiles []string) {
	os.MkdirAll(decodeLocation, os.ModePerm)
	var decodedFiles []string

	for _, encodedFile := range encodedFiles {
		fileName := decodeLocation + "/" + filepath.Base(encodedFile)
		decodedFiles = append(decodedFiles, fileName)

		cmd := exec.Command("base64", "-d", encodedFile, ">", fileName)
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error decoding file: %v\n", err)
		}
	}

	// Combine decoded file paths into a single string
	decodedFilesStr := strings.Join(decodedFiles, ":")

	os.Setenv("AZMON_METRIC_ACCOUNTS_AKV_FILES", decodedFilesStr)
}

func startMetricsExtension() {
	cmd := exec.Command(
		"/usr/sbin/MetricsExtension",
		"-Logger", "File",
		"-LogLevel", "Info",
		"-DataDirectory", "/opt/MetricsExtensionData",
		"-Input", "otlp_grpc_prom",
		"-PfxFile", os.Getenv("AZMON_METRIC_ACCOUNTS_AKV_FILES"),
		"-MonitoringAccount", os.Getenv("AZMON_DEFAULT_METRIC_ACCOUNT_NAME"),
		"-ConfigOverridesFilePath", os.Getenv("ME_CONFIG_FILE"),
	)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MetricsExtension: %v\n", err)
	}
}

func setEnvVarsFromEnvMdsdFile(envMdsdFile string) {
	file, err := os.Open(envMdsdFile)
	if err != nil {
		fmt.Printf("Error opening envmdsd file: %v\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		err := os.Setenv(line, "1")
		if err != nil {
			fmt.Printf("Error setting environment variable from envmdsd file: %v\n", err)
		}
	}
}

func startMdsd(mdsdLog string) {
	cmd := exec.Command(
		"mdsd",
		"-a", "-A",
		"-e", mdsdLog+"/mdsd.err",
		"-w", mdsdLog+"/mdsd.warn",
		"-o", mdsdLog+"/mdsd.info",
		"-q", mdsdLog+"/mdsd.qos",
		"2>>", "/dev/null",
	)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MDSD: %v\n", err)
	}
}

func printMdsdVersion() {
	cmd := exec.Command("mdsd", "--version")
	output, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error getting MDSD version: %v\n", err)
		return
	}
	fmt.Print(string(output))
}

func readMeConfigFileAsString(meConfigFile string) string {
	content, err := ioutil.ReadFile(meConfigFile)
	if err != nil {
		fmt.Printf("Error reading ME config file: %v\n", err)
		return ""
	}
	return string(content)
}

func startMetricsExtensionWithConfigOverrides(configOverrides string) {
	cmd := exec.Command(
		"/usr/sbin/MetricsExtension",
		"-Logger", "File",
		"-LogLevel", "Info",
		"-LocalControlChannel",
		"-TokenSource", "AMCS",
		"-DataDirectory", "/etc/mdsd.d/config-cache/metricsextension",
		"-Input", "otlp_grpc_prom",
		"-ConfigOverrides", configOverrides,
	)
	err := cmd.Start()
	if err != nil {
		fmt.Printf("Error starting MetricsExtension with configOverrides: %v\n", err)
	}
}

func readVersionFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func fmtVar(name, value string) {
	fmt.Printf("%s=\"%s\"\n", name, value)
}
