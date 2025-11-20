package utils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type secureTransport struct {
	underlyingTransport http.RoundTripper
	apiToken            string
}

/*
 * The secure RoundTrip with proper certificate validation
 */
func (t *secureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", t.apiToken))
	return t.underlyingTransport.RoundTrip(req)
}

/*
 * Create a custom HTTP transport with proper certificate configuration
 */
func CreateSecureTransport(token string) *secureTransport {
	// Create a custom TLS config that uses system certificates + any custom ones
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12, // Ensure modern TLS
	}

	// Load system certificate pool
	certPool, err := x509.SystemCertPool()
	if err != nil {
		fmt.Printf("Warning: Could not load system cert pool: %v\n", err)
		certPool = x509.NewCertPool()
	}

	// Check for custom certificate file (set by PowerShell script)
	certFile := os.Getenv("SSL_CERT_FILE")
	if certFile != "" {
		fmt.Printf("Loading additional certificates from: %s\n", certFile)
		certs, err := ioutil.ReadFile(certFile)
		if err == nil {
			if ok := certPool.AppendCertsFromPEM(certs); !ok {
				fmt.Printf("WARNING: Could not parse certificates from %s\n", certFile)
			} else {
				fmt.Printf("SUCCESS: Successfully loaded additional certificates\n")
			}
		} else {
			fmt.Printf("WARNING: Could not read certificate file: %v\n", err)
		}
	} else {
		fmt.Println("No additional SSL_CERT_FILE specified, using system certificates only.")
	}

	tlsConfig.RootCAs = certPool

	// Create transport with custom TLS config
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &secureTransport{
		underlyingTransport: transport,
		apiToken:            token,
	}
}
