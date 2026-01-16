/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package utils for common utils
package utils

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// GetDescName parse metrics name from prometheus.Desc object
func GetDescName(desc *prometheus.Desc) string {
	if desc == nil {
		return ""
	}
	str := desc.String()
	startIndex := strings.Index(str, "fqName: ")
	if startIndex == -1 {
		return ""
	}
	readfqName := str[startIndex+len("fqName: "):]

	endIndex := strings.Index(readfqName, ",")
	if endIndex == -1 {
		return ""
	}
	readfqName = readfqName[:endIndex]

	readfqName = strings.Trim(readfqName, "\"")
	return readfqName
}

// DoUpdateTelegraf update telegraf
func DoUpdateTelegraf(fieldMap map[string]interface{}, desc *prometheus.Desc, value interface{}, extInfo string) {
	if fieldMap == nil {
		return
	}
	fieldMap[GetDescName(desc)+extInfo] = value
}
