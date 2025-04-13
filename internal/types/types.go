package types

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ResourceClaimInfo struct {
	Name              string                `json:"name"`
	NodeName          string                `json:"node_name"`
	CreationTimestamp v1.Time               `json:"creation_timestamp"`
	Namespace         string                `json:"namespace"`
	Devices           []ResourceClaimDevice `json:"devices"`
	UsedByPod         bool                  `json:"used_by_pod"`
}

type ResourceClaimDevice struct {
	Name  string `json:"name"`
	Model string `json:"model"`
	State string `json:"state"`
}

type ResourceSliceInfo struct {
	Name              string                `json:"name"`
	NodeName          string                `json:"node_name"`
	CreationTimestamp v1.Time               `json:"creation_timestamp"`
	Driver            string                `json:"driver"`
	FabricID          string                `json:"fabric_id"`
	FabricModel       string                `json:"fabric_model"`
	Devices           []ResourceSliceDevice `json:"devices"`
}

type ResourceSliceDevice struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type NodeInfo struct {
	Name        string `json:"name"`
	FabricID    string `json:"fabric_id"`
	FabricModel string `json:"fabric_model"`
	Max         int    `json:"max"`
	Min         int    `json:"min"`
}
