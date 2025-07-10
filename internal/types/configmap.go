/*
Copyright 2025 The CoHDI Authors.

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

package types

type ComposableDRASpec struct {
	DeviceInfos   []DeviceInfo `json:"device-info"`
	LabelPrefix   string       `json:"label-prefix"`
	FabricIDRange []int        `json:"fabric-id-range"`
}

type DeviceInfo struct {
	Index             int               `json:"index"`
	CDIModelName      string            `json:"cdi-model-name"`
	DRAAttributes     map[string]string `json:"dra-attributes"`
	LabelKeyModel     string            `json:"label-key-model"`
	DriverName        string            `json:"driver-name"`
	K8sDeviceName     string            `json:"k8s-device-name"`
	CannotCoexistWith []int             `json:"cannot-coexist-with"`
}
