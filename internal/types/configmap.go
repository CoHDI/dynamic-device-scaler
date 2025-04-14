package types

type ComposableDRASpec struct {
	DeviceInfo    []DeviceInfo `yaml:"device-info"`
	LabelPrefix   string       `yaml:"label-prefix"`
	FabricIDRange []int        `yaml:"fabric-id-range"`
}

type DeviceInfo struct {
	Index             int           `yaml:"index"`
	CDIModelName      string        `yaml:"cdi-model-name"`
	DRAttributes      []DRAttribute `yaml:"dra-attributes"`
	LabelKeyModel     string        `yaml:"label-key-model"`
	DriverName        string        `yaml:"driver-name"`
	K8sDeviceName     string        `yaml:"k8s-device-name"`
	CannotCoexistWith []int         `yaml:"cannot-coexist-with"`
}

type DRAttribute struct {
	ProductName string `yaml:"productName"`
}
