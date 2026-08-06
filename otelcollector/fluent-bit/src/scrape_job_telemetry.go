package main

import "strings"

const (
	basicAuthEnabled                     = "BasicAuthEnabled"
	bearerTokenEnabledWithFile           = "BearerTokenEnabledWithFile"
	bearerTokenEnabledWithSecret         = "BearerTokenEnabledWithSecret"
	serviceMonitorBearerTokenFileEnabled = "ServiceMonitorBearerTokenFileEnabled"
	podMonitorBearerTokenFileEnabled     = "PodMonitorBearerTokenFileEnabled"
	serviceMonitorBearerTokenEnabled     = "ServiceMonitorBearerTokenEnabled"
	podMonitorBearerTokenEnabled         = "PodMonitorBearerTokenEnabled"
	serviceMonitorBasicAuthEnabled       = "ServiceMonitorBasicAuthEnabled"
	podMonitorBasicAuthEnabled           = "PodMonitorBasicAuthEnabled"
)

func scrapeJobMetadata(scrapeJobs map[string]interface{}) map[string]string {
	telemetryProperties := map[string]string{
		basicAuthEnabled:                     "false",
		bearerTokenEnabledWithFile:           "false",
		bearerTokenEnabledWithSecret:         "false",
		serviceMonitorBearerTokenFileEnabled: "false",
		podMonitorBearerTokenFileEnabled:     "false",
		serviceMonitorBearerTokenEnabled:     "false",
		podMonitorBearerTokenEnabled:         "false",
		serviceMonitorBasicAuthEnabled:       "false",
		podMonitorBasicAuthEnabled:           "false",
	}

	for jobName, value := range scrapeJobs {
		job, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		isServiceMonitor := strings.HasPrefix(jobName, "serviceMonitor/")
		isPodMonitor := strings.HasPrefix(jobName, "podMonitor/")

		if _, ok := job["basic_auth"]; ok {
			telemetryProperties[basicAuthEnabled] = "true"
			switch {
			case isServiceMonitor:
				telemetryProperties[serviceMonitorBasicAuthEnabled] = "true"
			case isPodMonitor:
				telemetryProperties[podMonitorBasicAuthEnabled] = "true"
			}
		}

		auth, ok := job["authorization"].(map[string]interface{})
		if !ok || auth["type"] != "Bearer" {
			continue
		}
		switch {
		case isServiceMonitor:
			telemetryProperties[serviceMonitorBearerTokenEnabled] = "true"
		case isPodMonitor:
			telemetryProperties[podMonitorBearerTokenEnabled] = "true"
		}
		if _, ok := auth["credentials"]; ok {
			telemetryProperties[bearerTokenEnabledWithSecret] = "true"
		}
		if _, ok := auth["credentials_file"]; !ok {
			continue
		}

		telemetryProperties[bearerTokenEnabledWithFile] = "true"
		switch {
		case isServiceMonitor:
			telemetryProperties[serviceMonitorBearerTokenFileEnabled] = "true"
		case isPodMonitor:
			telemetryProperties[podMonitorBearerTokenFileEnabled] = "true"
		}
	}

	return telemetryProperties
}
