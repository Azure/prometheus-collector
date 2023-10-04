// package prometheuscollector

// import (
//     "fmt"
// 	"net/http"
//     "os"
//     "os/exec"
//     "strings"
// )

// func main() {
//     // Run logging utility
//     // runShellCommand("source", "/opt/logger.sh")

//     // Run inotify as a daemon to track changes to the mounted configmap.
//     runShellCommand("touch", "/opt/inotifyoutput.txt")
//     runShellCommand("inotifywait", "/etc/config/settings", "--daemon", "--recursive", "--outfile", "/opt/inotifyoutput.txt", "--event", "create,delete", "--format", "'%e : %T'", "--timefmt", "'+%s'")

//     // Run ARC EULA utility
//     // runShellCommand("source", "/opt/arc-eula.sh")

//     // Check and set default values for environment variables
//     mode := os.Getenv("MODE")
//     if mode == "" {
//         mode = "simple"
//     }
//     controllerType := os.Getenv("CONTROLLER_TYPE")
//     cluster := os.Getenv("CLUSTER")

//     echoVar("MODE", mode)
//     echoVar("CONTROLLER_TYPE", controllerType)
//     echoVar("CLUSTER", cluster)

//     aikey := os.Getenv("APPLICATIONINSIGHTS_AUTH")
//     aikeyDecoded, _ := base64Decode(aikey)
//     os.Setenv("TELEMETRY_APPLICATIONINSIGHTS_KEY", aikeyDecoded)
//     fmt.Printf("export TELEMETRY_APPLICATIONINSIGHTS_KEY=%s\n", aikeyDecoded)

//     // get controller kind in lowercase, trimmed
//     controllerType = strings.ToLower(controllerType)
//     controllerType = strings.TrimSpace(controllerType)

//     // If using a trusted CA for HTTP Proxy, copy this over from the node and install
//     runShellCommand("cp", "/anchors/ubuntu/*", "/etc/pki/ca-trust/source/anchors", "2>/dev/null")
//     runShellCommand("cp", "/anchors/mariner/*", "/etc/pki/ca-trust/source/anchors", "2>/dev/null")
//     runShellCommand("cp", "/anchors/proxy/*", "/etc/pki/ca-trust/source/anchors", "2>/dev/null")
//     runShellCommand("update-ca-trust")

//     // Set environment variables for proxy
//     setProxyEnvironmentVariables("http_proxy")
//     setProxyEnvironmentVariables("HTTP_PROXY")
//     setProxyEnvironmentVariables("https_proxy")
//     setProxyEnvironmentVariables("HTTPS_PROXY")

//     // Add target-allocator service to the no_proxy env variable
//     addNoProxy("ama-metrics-operator-targets.kube-system.svc.cluster.local")

//     // Set HTTP_PROXY_ENABLED
//     setHTTPProxyEnabled()

//     // If ARC_CLUSTER is true and HTTP_PROXY_ENABLED is true, configure proxy settings
//     if os.Getenv("IS_ARC_CLUSTER") == "true" && os.Getenv("HTTP_PROXY_ENABLED") == "true" {
//         configureProxy()
//     }

//     // Run configmap-parser.sh
//     runShellCommand("source", "/opt/configmap-parser.sh")

//     // Start cron daemon for logrotate
//     runShellCommand("/usr/sbin/crond", "-n", "-s", "&")

//     if controllerType == "replicaset" {
//         fluentBitConfigFile := "/opt/fluent-bit/fluent-bit.conf"
//         meConfigFile := "/usr/sbin/me.config"

//         if os.Getenv("CLUSTER_OVERRIDE") == "true" {
//             meConfigFile = "/usr/sbin/me_internal.config"
//         }

//     } else {
//         fluentBitConfigFile := "/opt/fluent-bit/fluent-bit-daemonset.conf"
//         meConfigFile := "/usr/sbin/me_ds.config"

//         if os.Getenv("CLUSTER_OVERRIDE") == "true" {
//             meConfigFile = "/usr/sbin/me_ds_internal.config"
//         }
//     }

//     if os.Getenv("MAC") == "true" {
//         // Wait for addon-token-adapter to be healthy
//         // Implement this part using Go code

//     }

//     os.Setenv("ME_CONFIG_FILE", meConfigFile)
//     os.Setenv("FLUENT_BIT_CONFIG_FILE", fluentBitConfigFile)

//     // Start MetricsExtension
//     // Implement this part using Go code

//     // Get ME version
//     meVersion := readFileContents("/opt/metricsextversion.txt")
//     echoVar("ME_VERSION", meVersion)

//     // Get ruby version
//     rubyVersion := runShellCommandAndGetOutput("ruby", "--version")
//     echoVar("RUBY_VERSION", rubyVersion)

//     // Get golang version
//     goVersion := readFileContents("/opt/goversion.txt")
//     echoVar("GOLANG_VERSION", goVersion)

//     // Start otelcollector
//     // Implement this part using Go code

//     // Get OTELCOLLECTOR_VERSION and PROMETHEUS_VERSION
//     otelCollectorVersion := runShellCommandAndGetOutput("/opt/microsoft/otelcollector/otelcollector", "--version")
//     prometheusVersion := readFileContents("/opt/microsoft/otelcollector/PROMETHEUS_VERSION")
//     echoVar("OTELCOLLECTOR_VERSION", otelCollectorVersion)
//     echoVar("PROMETHEUS_VERSION", prometheusVersion)

//     fmt.Println("starting telegraf")
//     if os.Getenv("TELEMETRY_DISABLED") != "true" {
//         // Implement this part using Go code
//     }

//     fmt.Println("starting fluent-bit")
//     os.Mkdir("/opt/microsoft/fluent-bit", os.ModePerm)
//     os.Create("/opt/microsoft/fluent-bit/fluent-bit-out-appinsights-runtime.log")
//     fluentBitVersion := runShellCommandAndGetOutput("fluent-bit", "--version")
//     echoVar("FLUENT_BIT_VERSION", fluentBitVersion)
//     echoVar("FLUENT_BIT_CONFIG_FILE", fluentBitConfigFile)

//     if os.Getenv("MAC") == "true" {
//         // Implement this part using Go code
//     }

//     // Setting time at which the container started running
//     epochTimeNow := fmt.Sprintf("%d", time.Now().Unix())
//     writeFileContents("/opt/microsoft/liveness/azmon-container-start-time", epochTimeNow)
//     echoVar("AZMON_CONTAINER_START_TIME", epochTimeNow)
//     epochTimeNowReadable := time.Now().Format(time.RFC3339)
//     echoVar("AZMON_CONTAINER_START_TIME_READABLE", epochTimeNowReadable)

//     // Expose a health endpoint for liveness probe
//     http.HandleFunc("/health", healthHandler)
//     http.ListenAndServe(":8080", nil)
// }

// func runShellCommand(command string, args ...string) {
//     cmd := exec.Command(command, args...)
//     cmd.Stdout = os.Stdout
//     cmd.Stderr = os.Stderr
//     cmd.Run()
// }

// func runShellCommandAndGetOutput(command string, args ...string) string {
//     cmd := exec.Command(command, args...)
//     output, err := cmd.Output()
//     if err != nil {
//         fmt.Println("Error:", err)
//         return ""
//     }
//     return string(output)
// }

// func echoVar(name, value string) {
//     fmt.Printf("echo_var \"%s\" \"%s\"\n", name, value)
// }

// func base64Decode(input string) (string, error) {
//     decodedBytes, err := base64.StdEncoding.DecodeString(input)
//     if err != nil {
//         return "", err
//     }
//     return string(decodedBytes), nil
// }

// func setProxyEnvironmentVariables(envVarName string) {
//     proxy := os.Getenv(envVarName)
//     if proxy != "" && strings.HasSuffix(proxy, "/") {
//         os.Setenv(envVarName, proxy[:len(proxy)-1])
//     }
// }

// func addNoProxy(host string) {
//     noProxy := os.Getenv("NO_PROXY")
//     noProxy = strings.TrimSuffix(noProxy, ",")
//     noProxy += "," + host
//     os.Setenv("NO_PROXY", noProxy)
// }

// func setHTTPProxyEnabled() {
//     httpProxy := os.Getenv("HTTP_PROXY")
//     httpProxyEnabled := "false"
//     if httpProxy != "" {
//         httpProxyEnabled = "true"
//     }
//     os.Setenv("HTTP_PROXY_ENABLED", httpProxyEnabled)
// }

// func configureProxy() {
//     // Implement this part using Go code
// }

// func readFileContents(filePath string) string {
//     content, err := os.ReadFile(filePath)
//     if err != nil {
//         fmt.Println("Error reading file:", err)
//         return ""
//     }
//     return string(content)
// }

// func writeFileContents(filePath, content string) {
//     err := os.WriteFile(filePath, []byte(content), os.ModePerm)
//     if err != nil {
//         fmt.Println("Error writing file:", err)
//     }
// }

// func healthHandler(w http.ResponseWriter, r *http.Request) {
//     otelCollectorRunning := isProcessRunning("otelcollector.exe")
//     // metricsExtensionRunning := isProcessRunning("metricsextension.exe")

//     if otelCollectorRunning && metricsExtensionRunning {
//         w.WriteHeader(http.StatusOK)
//         fmt.Fprintln(w, "Both otelcollector.exe and metricsextension.exe are running.")
//     } else {
//         w.WriteHeader(http.StatusServiceUnavailable)
//         fmt.Fprintln(w, "One or both of the processes are not running.")
//     }
// }

// func isProcessRunning(processName string) bool {
//     cmd := exec.Command("pgrep", processName)
//     err := cmd.Run()
//     return err == nil
// }
