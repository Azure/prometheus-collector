package certificates

import "time"

const (
	// DefaultValidityYears is the duration for regular certificates, SSL etc. 2 years.
	ServerValidityYears = 2

	// CaValidityYears is the duration for CA certificates. 30 years.
	CaValidityYears = 30

	// ClockSkewDuration is the allowed clock skews.
	ClockSkewDuration = time.Minute * 10

	// KeyRetryCount is the number of retries for certificate generation.
	KeyRetryCount    = 3
	KeyRetryInterval = time.Microsecond * 5
	KeyRetryTimeout  = time.Second * 10
)
