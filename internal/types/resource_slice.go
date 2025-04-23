package types

import v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ResourceSliceState string

const (
	// ResourceSliceStateRed indicates that the resource is attached to the node
	ResourceSliceStateRed ResourceSliceState = "Red"
	// ResourceSliceStateGreen indicates that the resource is not attached to the node
	ResourceSliceStateGreen ResourceSliceState = "Green"
)

type ResourceSliceInfo struct {
	Name              string                `json:"name"`
	State             ResourceSliceState    `json:"state"`
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
