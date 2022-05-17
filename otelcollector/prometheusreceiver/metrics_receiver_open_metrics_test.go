// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const testDir = "./testdata/openmetrics/"

// nolint:unused
var skippedTests = map[string]struct{}{
	"bad_clashing_names_0": {}, "bad_clashing_names_1": {}, "bad_clashing_names_2": {},
	"bad_counter_values_0": {}, "bad_counter_values_1": {}, "bad_counter_values_2": {},
	"bad_counter_values_3": {}, "bad_counter_values_5": {}, "bad_counter_values_6": {},
	"bad_counter_values_10": {}, "bad_counter_values_11": {}, "bad_counter_values_12": {},
	"bad_counter_values_13": {}, "bad_counter_values_14": {}, "bad_counter_values_15": {},
	"bad_counter_values_16": {}, "bad_counter_values_17": {}, "bad_counter_values_18": {},
	"bad_counter_values_19": {}, "bad_exemplars_on_unallowed_samples_2": {}, "bad_exemplar_timestamp_0": {},
	"bad_exemplar_timestamp_1": {}, "bad_exemplar_timestamp_2": {}, "bad_grouping_or_ordering_0": {},
	"bad_grouping_or_ordering_2": {}, "bad_grouping_or_ordering_3": {}, "bad_grouping_or_ordering_4": {},
	"bad_grouping_or_ordering_5": {}, "bad_grouping_or_ordering_6": {}, "bad_grouping_or_ordering_7": {},
	"bad_grouping_or_ordering_8": {}, "bad_grouping_or_ordering_9": {}, "bad_grouping_or_ordering_10": {},
	"bad_histograms_0": {}, "bad_histograms_1": {}, "bad_histograms_2": {}, "bad_histograms_3": {},
	"bad_histograms_6": {}, "bad_histograms_7": {}, "bad_histograms_8": {},
	"bad_info_and_stateset_values_0": {}, "bad_info_and_stateset_values_1": {}, "bad_metadata_in_wrong_place_0": {},
	"bad_metadata_in_wrong_place_1": {}, "bad_metadata_in_wrong_place_2": {},
	"bad_missing_or_invalid_labels_for_a_type_1": {}, "bad_missing_or_invalid_labels_for_a_type_3": {},
	"bad_missing_or_invalid_labels_for_a_type_4": {}, "bad_missing_or_invalid_labels_for_a_type_6": {},
	"bad_missing_or_invalid_labels_for_a_type_7": {}, "bad_repeated_metadata_0": {},
	"bad_repeated_metadata_1": {}, "bad_repeated_metadata_3": {}, "bad_stateset_info_values_0": {},
	"bad_stateset_info_values_1": {}, "bad_stateset_info_values_2": {}, "bad_stateset_info_values_3": {},
	"bad_timestamp_4": {}, "bad_timestamp_5": {}, "bad_timestamp_7": {}, "bad_unit_6": {}, "bad_unit_7": {},
}

func verifyPositiveTarget(t *testing.T, _ *testData, mds []*pmetric.ResourceMetrics) {
	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 1, metrics)
}

// Test open metrics positive test cases
func TestOpenMetricsPositive(t *testing.T) {
	targetsMap := getOpenMetricsTestData(false)
	targets := make([]*testData, 0)
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyPositiveTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, false, "")
}

// nolint:unused
func verifyNegativeTarget(t *testing.T, td *testData, mds []*pmetric.ResourceMetrics) {
	// failing negative tests are skipped since prometheus scrape package is currently not fully
	// compatible with OpenMetrics tests and successfully scrapes some invalid metrics
	// see: https://github.com/prometheus/prometheus/issues/9699
	if _, ok := skippedTests[td.name]; ok {
		t.Skip("skipping failing negative OpenMetrics parser tests")
	}

	require.Greater(t, len(mds), 0, "At least one resource metric should be present")
	metrics := getMetrics(mds[0])
	assertUp(t, 0, metrics)
}

// Test open metrics negative test cases
func TestOpenMetricsNegative(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9119")

	targetsMap := getOpenMetricsTestData(true)
	targets := make([]*testData, 0)
	for k, v := range targetsMap {
		testData := &testData{
			name: k,
			pages: []mockPrometheusResponse{
				{code: 200, data: v, useOpenMetrics: true},
			},
			validateFunc:    verifyNegativeTarget,
			validateScrapes: true,
		}
		targets = append(targets, testData)
	}

	testComponent(t, targets, false, "")
}

//reads test data from testdata/openmetrics directory
func getOpenMetricsTestData(negativeTestsOnly bool) map[string]string {
	testDir, err := os.Open(testDir)
	if err != nil {
		log.Fatalf("failed opening openmetrics test directory")
	}
	defer testDir.Close()

	//read all test file names in testdata/openmetrics
	testList, _ := testDir.Readdirnames(0)

	targetsData := make(map[string]string)
	for _, testName := range testList {
		//ignore hidden files
		if strings.HasPrefix(testName, ".") {
			continue
		}
		if negativeTestsOnly && !strings.Contains(testName, "bad") {
			continue
		} else if !negativeTestsOnly && strings.Contains(testName, "bad") {
			continue
		}
		if testData, err := readTestCase(testName); err == nil {
			targetsData[testName] = testData
		}
	}
	return targetsData
}

func readTestCase(testName string) (string, error) {
	filePath := fmt.Sprintf("%s/%s/metrics", testDir, testName)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("failed opening file: %s", filePath)
		return "", err
	}
	return string(content), nil
}
