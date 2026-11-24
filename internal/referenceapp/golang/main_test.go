package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestReferenceAppBasicAuthPort(t *testing.T) {
	if os.Getenv("REFAPP_TEST_CHILD") == "1" {
		main()
		return
	}

	occupied, err := net.Listen("tcp", ":2114")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	available, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := available.Addr().(*net.TCPAddr).Port
	available.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^TestReferenceAppBasicAuthPort$")
	cmd.Env = append(os.Environ(), "REFAPP_TEST_CHILD=1", fmt.Sprintf("BASIC_AUTH_PORT=%d", port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	client := &http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/httpsmetrics", port)
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth("admin", "pwd")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("basic auth endpoint returned %s", resp.Status)
		}
		return
	}
	t.Fatal("basic auth endpoint did not start while port 2114 was occupied")
}
