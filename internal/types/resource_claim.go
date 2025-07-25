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

type ResourceClaimInfo struct {
	Name              string                `json:"name"`
	NodeName          string                `json:"node_name"`
	CreationTimestamp v1.Time               `json:"creation_timestamp"`
	Namespace         string                `json:"namespace"`
	Devices           []ResourceClaimDevice `json:"devices"`
}

type ResourceClaimDevice struct {
	Name              string `json:"name"`
	Model             string `json:"model"`
	State             string `json:"state"`
	ResourceSliceName string `json:"resource_slice_name"`
}
