package configprocessing

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	"prometheus-collector/otelcollector/test/utils"
)

func monitorTransitionState(jobs []string, username string, absent bool) error {
	responses, err := utils.QueryCollectorPromUI("/api/v1/status/config")
	if err != nil {
		return err
	}
	for _, response := range responses {
		var data promv1.ConfigResult
		if err := json.Unmarshal(response.Data, &data); err != nil {
			return err
		}
		configs, err := utils.ParseScrapeJobs[*config.ScrapeConfig](data.YAML)
		if err != nil {
			return err
		}
		if err := validateMonitorTransitionState(configs, jobs, username, absent); err != nil {
			return fmt.Errorf("pod %s: %w", response.PodName, err)
		}
	}
	return nil
}

func validateMonitorTransitionState(configs []*config.ScrapeConfig, jobs []string, username string, absent bool) error {
	for _, expected := range jobs {
		found := false
		for _, job := range configs {
			if job.JobName != expected {
				continue
			}
			found = true
			if !absent && (job.HTTPClientConfig.BasicAuth == nil ||
				job.HTTPClientConfig.BasicAuth.Username != username) {
				return fmt.Errorf("unexpected dummy username for %s", expected)
			}
		}
		if found == absent {
			return fmt.Errorf("%s presence=%t, expected absence=%t", expected, found, absent)
		}
	}
	return nil
}

type annotationExpectation struct {
	Regex      string
	Namespaces []string
}

func validateAnnotationConfig(jobs []*config.ScrapeConfig, expression string) error {
	for _, job := range jobs {
		if job.JobName != "kubernetes-pods" {
			continue
		}
		if expression == "" {
			return fmt.Errorf("removed annotation job remains configured")
		}
		for _, rule := range job.RelabelConfigs {
			if rule.Action == "keep" && len(rule.SourceLabels) == 1 &&
				rule.SourceLabels[0] == "__meta_kubernetes_namespace" && rule.Regex.String() == expression {
				return nil
			}
		}
		return fmt.Errorf("annotation namespace selector has not converged")
	}
	if expression != "" {
		return fmt.Errorf("admitted annotation job missing from config")
	}
	return nil
}

func validateMonitorJobs(jobs, allowed, denied []string) error {
	found := func(prefix string) bool {
		for _, job := range jobs {
			if strings.HasPrefix(job, prefix+"/") {
				return true
			}
		}
		return false
	}
	for _, prefix := range allowed {
		if !found(prefix) {
			return fmt.Errorf("allowed control %s was not reconciled", prefix)
		}
	}
	for _, prefix := range denied {
		if found(prefix) {
			return fmt.Errorf("disallowed monitor %s was generated", prefix)
		}
	}
	return nil
}

func expectMonitorJobs(allowed, denied []string) {
	check := func() error {
		responses, err := utils.QueryCollectorPromUI("/api/v1/status/config")
		if err != nil {
			return err
		}
		for _, response := range responses {
			var data promv1.ConfigResult
			if err := json.Unmarshal(response.Data, &data); err != nil {
				return err
			}
			configs, err := utils.ParseScrapeJobs[*config.ScrapeConfig](data.YAML)
			if err != nil {
				return err
			}
			jobs := make([]string, 0, len(configs))
			for _, job := range configs {
				jobs = append(jobs, job.JobName)
			}
			if err := validateMonitorJobs(jobs, allowed, denied); err != nil {
				return fmt.Errorf("pod %s: %w", response.PodName, err)
			}
		}
		return nil
	}
	Eventually(check, 5*time.Minute, 15*time.Second).Should(Succeed())
	Consistently(check, time.Minute, 15*time.Second).Should(Succeed())
}

func expectAuthRecovered(api promv1.API, cluster string, jobs, metrics []string, after time.Time) {
	Eventually(func() error { return monitorTransitionState(jobs, "admin", false) },
		8*time.Minute, 15*time.Second).Should(Succeed())
	Eventually(func() error {
		targets, err := utils.TransitionTargets()
		if err != nil {
			return err
		}
		return utils.ValidateTransitionTargets(targets, jobs, "up", after)
	}, 8*time.Minute, 15*time.Second).Should(Succeed())
	for _, job := range metrics {
		utils.ExpectTransitionMetric(api, "my_metric", job, cluster, "", after)
	}
}

func expectAnnotationNamespaces(api promv1.API, cluster string, expected annotationExpectation, after time.Time) {
	checkTargets := func() error {
		responses, err := utils.QueryCollectorPromUI("/api/v1/status/config")
		if err != nil {
			return err
		}
		for _, response := range responses {
			var data promv1.ConfigResult
			if err := json.Unmarshal(response.Data, &data); err != nil {
				return err
			}
			jobs, err := utils.ParseScrapeJobs[*config.ScrapeConfig](data.YAML)
			if err != nil {
				return err
			}
			if err := validateAnnotationConfig(jobs, expected.Regex); err != nil {
				return err
			}
		}
		targets, err := utils.TransitionTargets()
		if err != nil {
			return err
		}
		return utils.ValidateAnnotationTargets(targets, expected.Namespaces, after)
	}
	Eventually(checkTargets, 8*time.Minute, 15*time.Second).Should(Succeed())
	after = time.Now()
	AddReportEntry("annotation-observation-start", after.UTC().Format(time.RFC3339Nano))
	check := func() error {
		if err := checkTargets(); err != nil {
			return err
		}
		for _, namespace := range []string{"ama-settings-included", "ama-settings-excluded", "ama-settings-included-similar"} {
			selector := fmt.Sprintf("weather_temperature{job=%q,cluster=%q,kubernetes_namespace=%q,kubernetes_pod_name=%q}",
				"kubernetes-pods", cluster, namespace, "annotation-target")
			samples, err := utils.FreshTransitionSamples(api, selector, after)
			if err != nil {
				return err
			}
			if slices.Contains(expected.Namespaces, namespace) {
				if err := utils.ValidateMetricSamples(samples, "weather_temperature", "kubernetes-pods", cluster, time.Now(), false); err != nil {
					return err
				}
			} else if len(samples) != 0 {
				return fmt.Errorf("excluded annotation namespace %s still ingests new samples", namespace)
			}
		}
		samples, err := utils.FreshTransitionSamples(api,
			fmt.Sprintf("weather_temperature{job=%q,cluster=%q,kubernetes_pod_name=%q}",
				"kubernetes-pods", cluster, "unannotated-target"), after)
		if err != nil {
			return err
		}
		if len(samples) != 0 {
			return fmt.Errorf("unannotated negative control ingests new samples")
		}
		return nil
	}
	Eventually(check, 8*time.Minute, 15*time.Second).Should(Succeed())
	Consistently(check, time.Minute, 15*time.Second).Should(Succeed())
}
