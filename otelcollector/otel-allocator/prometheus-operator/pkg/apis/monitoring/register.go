// Copyright 2018 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monitoring

import "os"

// const (
// 	GroupName = "monitoring.coreos.com"
// )

const GroupName = func() string {
	group := "monitoring.coreos.com"
	customGroupV1 := os.Getenv("PROMETHEUS_OPERATOR_V1_CUSTOM_GROUP")
	if customGroupV1 != "" {
		group = customGroupV1
	}
	return group
}()
