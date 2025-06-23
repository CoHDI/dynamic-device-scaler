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
