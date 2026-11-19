package configmapsettings

import (
	"os"
	"path/filepath"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("When parsing the deny_fs_access_through_sms setting", func() {
	populate := func(settings map[string]string) *ConfigProcessor {
		cp := &ConfigProcessor{}
		cp.PopulateSettingValuesFromConfigMap(map[string]map[string]string{
			"prometheus-collector-settings": settings,
		})
		return cp
	}

	writeEnvFile := func(cp *ConfigProcessor) string {
		path := filepath.Join(ginkgo.GinkgoT().TempDir(), "prometheus-collector-settings-env")
		gomega.Expect((&FileConfigWriter{}).WriteConfigToFile(path, cp)).To(gomega.Succeed())
		content, err := os.ReadFile(path)
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		return string(content)
	}

	ginkgo.DescribeTable("populating the setting",
		func(settings map[string]string, expected bool) {
			gomega.Expect(populate(settings).DenyFSAccessThroughSMs).To(gomega.Equal(expected))
		},
		ginkgo.Entry("is disabled when the key is absent", map[string]string{"cluster_alias": "x"}, false),
		ginkgo.Entry("is disabled when set to false", map[string]string{"deny_fs_access_through_sms": "false"}, false),
		ginkgo.Entry("is disabled when empty", map[string]string{"deny_fs_access_through_sms": ""}, false),
		ginkgo.Entry("is disabled for a non-boolean value", map[string]string{"deny_fs_access_through_sms": "yes"}, false),
		ginkgo.Entry("is enabled when set to true", map[string]string{"deny_fs_access_through_sms": "true"}, true),
		ginkgo.Entry("is enabled when set to TRUE with whitespace", map[string]string{"deny_fs_access_through_sms": " TRUE "}, true),
	)

	ginkgo.It("is disabled when the prometheus-collector-settings section is missing", func() {
		cp := &ConfigProcessor{}
		cp.PopulateSettingValuesFromConfigMap(map[string]map[string]string{})
		gomega.Expect(cp.DenyFSAccessThroughSMs).To(gomega.BeFalse())
	})

	ginkgo.It("exports AZMON_DENY_FS_ACCESS_THROUGH_SMS=true when enabled", func() {
		cp := populate(map[string]string{"deny_fs_access_through_sms": "true"})
		gomega.Expect(writeEnvFile(cp)).To(gomega.ContainSubstring("AZMON_DENY_FS_ACCESS_THROUGH_SMS=true\n"))
	})

	ginkgo.It("does not export AZMON_DENY_FS_ACCESS_THROUGH_SMS when disabled", func() {
		cp := populate(map[string]string{"deny_fs_access_through_sms": "false"})
		gomega.Expect(writeEnvFile(cp)).NotTo(gomega.ContainSubstring("AZMON_DENY_FS_ACCESS_THROUGH_SMS"))
	})

	ginkgo.It("does not affect other prometheus-collector-settings", func() {
		cp := populate(map[string]string{
			"deny_fs_access_through_sms": "true",
			"cluster_alias":              "my-cluster",
		})
		gomega.Expect(cp.DenyFSAccessThroughSMs).To(gomega.BeTrue())
		gomega.Expect(cp.ClusterAlias).To(gomega.Equal("my_cluster"))
	})
})
