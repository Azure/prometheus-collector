package ccpconfigmapsettings

import (
	"fmt"
	"strings"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

// tomlparserCCPTargetsMetricsKeepList processes the configuration and writes it to a file.
func tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	opts := cmcommon.KeepListOptions{
		Jobs:                  shared.ControlPlaneDefaultScrapeJobs,
		MetricsConfig:         metricsConfigBySection,
		SchemaVersion:         configSchemaVersion,
		KeepListSection:       "default-targets-metrics-keep-list",
		MinimalProfileSection: "minimal-ingestion-profile",
		MinimalProfileKeyV1:   "minimalingestionprofile",
		MinimalProfileKeyV2:   "enabled",
		OutputPath:            configMapKeepListEnvVarPath,
		EnvVarFormatter: func(jobName string) string {
			return fmt.Sprintf("CONTROLPLANE_%s_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		},
		JobKeyFormatter: func(jobName, schema string) string {
			if schema == shared.SchemaVersion.V1 {
				return "controlplane-" + jobName
			}
			return jobName
		},
		MinimalProfileDefaultBool: true,
	}

	if err := cmcommon.PopulateKeepLists(opts); err != nil {
		fmt.Println("tomlparserCCPTargetsMetricsKeepList::error populating keep list", err)
	}

	fmt.Println("End default-targets-metrics-keep-list Processing")
}
