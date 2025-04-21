package types

type NodeInfo struct {
	Name     string             `json:"name"`
	FabricID string             `json:"fabric_id"`
	Models   []ModelConstraints `json:"models"`
}

type ModelConstraints struct {
	Model      string `json:"model"`
	DeviceName string `json:"device_name"`
	MaxDevice  int    `json:"max_device"`
	MinDevice  int    `json:"min_device"`
}
