package types

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ResourceClaimInfo struct {
	Name              string   `json:"name"`
	NodeName          string   `json:"node_name"`
	CreationTimestamp v1.Time  `json:"creation_timestamp"`
	Namespace         string   `json:"namespace"`
	Devices           []Device `json:"devices"`
	UsedByPod         bool     `json:"used_by_pod"`
}

type Device struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	State string `json:"state"`
}

type ResourceSliceInfo struct {
}

type NodeInfo struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	Max   int    `json:"max"`
	Min   int    `json:"min"`
}
