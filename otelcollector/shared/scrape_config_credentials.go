package shared

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

func DropScrapeConfigsWithFileRefs(promConfigYAML string) (string, []string, error) {
	var config map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(promConfigYAML), &config); err != nil {
		return promConfigYAML, nil, fmt.Errorf("parsing prometheus config: %w", err)
	}

	jobs, _ := config["scrape_configs"].([]interface{})
	if len(jobs) == 0 {
		return promConfigYAML, nil, nil
	}

	kept := make([]interface{}, 0, len(jobs))
	var dropped []string
	for index, job := range jobs {
		if refs := credentialFileRefs(job); len(refs) > 0 {
			dropped = append(dropped, fmt.Sprintf("%s (%s)", scrapeJobName(job, index), strings.Join(refs, ", ")))
			continue
		}
		kept = append(kept, job)
	}
	if len(dropped) == 0 {
		return promConfigYAML, nil, nil
	}

	config["scrape_configs"] = kept
	filtered, err := yaml.Marshal(config)
	if err != nil {
		return promConfigYAML, nil, fmt.Errorf("marshalling filtered prometheus config: %w", err)
	}
	return string(filtered), dropped, nil
}

func credentialFileRefs(node any) []string {
	var refs []string
	collectCredentialFileRefs("", node, &refs)
	sort.Strings(refs)
	return refs
}

func scrapeJobName(job any, index int) string {
	if values, ok := job.(map[interface{}]interface{}); ok {
		if name, ok := values["job_name"].(string); ok && name != "" {
			return name
		}
	}
	return fmt.Sprintf("scrape_configs[%d]", index)
}

func collectCredentialFileRefs(path string, node any, refs *[]string) {
	visit := func(key string, value any) {
		child := key
		if path != "" {
			child = path + "." + key
		}
		if strings.HasSuffix(key, "_file") {
			*refs = append(*refs, child)
			return
		}
		collectCredentialFileRefs(child, value, refs)
	}

	switch values := node.(type) {
	case map[interface{}]interface{}:
		for key, value := range values {
			visit(fmt.Sprint(key), value)
		}
	case []interface{}:
		for index, value := range values {
			collectCredentialFileRefs(fmt.Sprintf("%s[%d]", path, index), value, refs)
		}
	}
}
