package configmapsettings

import (
	"fmt"
	"log"
	"strings"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

func tomlparserTargetsMetricsKeepList(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	shared.EchoSectionDivider("Start Processing - tomlparserTargetsMetricsKeepList")

	formatter := func(jobName string) string {
		return fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))
	}

	opts := cmcommon.KeepListOptions{
		Jobs:                      shared.DefaultScrapeJobs,
		MetricsConfig:             metricsConfigBySection,
		SchemaVersion:             configSchemaVersion,
		KeepListSection:           "default-targets-metrics-keep-list",
		MinimalProfileSection:     "minimal-ingestion-profile",
		MinimalProfileKeyV1:       "minimalingestionprofile",
		MinimalProfileKeyV2:       "enabled",
		OutputPath:                configMapKeepListEnvVarPath,
		EnvVarFormatter:           func(jobName string) string { return formatter(jobName) },
		JobKeyFormatter:           func(jobName, _ string) string { return jobName },
		MinimalProfileDefaultBool: true,
	}

	if err := cmcommon.PopulateKeepLists(opts); err != nil {
		log.Printf("tomlparserTargetsMetricsKeepList::error populating keep list: %s\n", err.Error())
	}

	shared.EchoSectionDivider("End Processing - tomlparserTargetsMetricsKeepList")
}
