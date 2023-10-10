package main

import (
    "fmt"
	"net/http"
    "os"
    "os/exec"
)

func main() {
    // Run inotify as a daemon to track changes to the mounted configmap.
    runShellCommand("touch", "/opt/inotifyoutput.txt")
    runShellCommand("inotifywait", "/etc/config/settings", "--daemon", "--recursive", "--outfile", "/opt/inotifyoutput.txt", "--event", "create,delete", "--format", "'%e : %T'", "--timefmt", "'+%s'")

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
    prometheuscollectorRunning := isProcessRunning("prometheuscollector.exe")

    if prometheuscollectorRunning {
        w.WriteHeader(http.StatusOK)
        fmt.Fprintln(w, "prometheuscollector.exe is running.")
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
        fmt.Fprintln(w, "prometheuscollector.exe is not running.")
    }
}

func isProcessRunning(processName string) bool {
    cmd := exec.Command("pgrep", processName)
    err := cmd.Run()
    return err == nil
}
