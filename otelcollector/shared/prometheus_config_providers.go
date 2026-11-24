package shared

import (
	"fmt"
	"strings"
)

const RestrictPrometheusConfigProvidersEnv = "AZMON_RESTRICT_PROMETHEUS_CONFIG_PROVIDERS"

func ValidatePrometheusConfigProviders(uri string, value any) error {
	if uri == "env:NODE_NAME" || uri == "env:NODE_IP" {
		if text, ok := value.(string); ok && strings.Contains(text, "$") {
			return fmt.Errorf("node environment provider value must not contain '$'")
		}
		return nil
	}

	root, ok := stringMap(value)
	if !ok {
		return checkProviderReferences(value, false)
	}

	if receivers, exists := root["receivers"]; exists {
		if err := checkPrometheusReceivers(receivers); err != nil {
			return err
		}
	}
	for key, child := range root {
		if isFlattenedPrometheusConfigKey(key) {
			if err := checkProviderReferences(child, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPrometheusReceivers(value any) error {
	receivers, ok := stringMap(value)
	if !ok {
		return checkProviderReferences(value, false)
	}
	for name, receiver := range receivers {
		if name != "prometheus" && !strings.HasPrefix(name, "prometheus/") {
			continue
		}
		if err := checkProviderReferences(receiver, true); err != nil {
			return err
		}
	}
	return nil
}

func checkProviderReferences(value any, allowNodeEnv bool) error {
	switch node := value.(type) {
	case string:
		for offset := 0; offset < len(node); {
			open := strings.Index(node[offset:], "${")
			if open < 0 {
				break
			}
			open += offset
			closeOffset := strings.IndexByte(node[open+2:], '}')
			if closeOffset < 0 {
				break
			}
			close := open + 2 + closeOffset

			precedingDollars := 0
			for i := open - 1; i >= 0 && node[i] == '$'; i-- {
				precedingDollars++
			}
			if precedingDollars%2 == 1 {
				offset = close + 1
				continue
			}

			provider, name, found := strings.Cut(node[open+2:close], ":")
			if found && (provider == "file" || provider == "env") {
				if allowNodeEnv && provider == "env" && (name == "NODE_NAME" || name == "NODE_IP") {
					offset = close + 1
					continue
				}
				return fmt.Errorf("tenant Prometheus configuration may not use file or env providers")
			}
			offset = close + 1
		}
	case map[string]any:
		for _, child := range node {
			if err := checkProviderReferences(child, allowNodeEnv); err != nil {
				return err
			}
		}
	case map[interface{}]interface{}:
		for _, child := range node {
			if err := checkProviderReferences(child, allowNodeEnv); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := checkProviderReferences(child, allowNodeEnv); err != nil {
				return err
			}
		}
	}
	return nil
}

func stringMap(value any) (map[string]any, bool) {
	switch node := value.(type) {
	case map[string]any:
		return node, true
	case map[interface{}]interface{}:
		result := make(map[string]any, len(node))
		for key, child := range node {
			result[fmt.Sprint(key)] = child
		}
		return result, true
	default:
		return nil, false
	}
}

func isFlattenedPrometheusConfigKey(key string) bool {
	for _, separator := range []string{"::", ".", "/"} {
		parts := strings.Split(key, separator)
		if len(parts) >= 3 && parts[0] == "receivers" &&
			(parts[1] == "prometheus" || strings.HasPrefix(parts[1], "prometheus/")) &&
			parts[2] == "config" {
			return true
		}
	}
	return strings.HasPrefix(key, "receivers/prometheus/") && strings.Contains(key, "/config/")
}
