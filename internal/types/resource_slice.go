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

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ResourceSliceInfo struct {
	Name              string                `json:"name"`
	NodeName          string                `json:"node_name"`
	CreationTimestamp v1.Time               `json:"creation_timestamp"`
	Driver            string                `json:"driver"`
	Devices           []ResourceSliceDevice `json:"devices"`
	Pool              string                `json:"pool"`
}

type ResourceSliceDevice struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}
