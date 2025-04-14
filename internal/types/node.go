package types

type NodeInfo struct {
	Name        string `json:"name"`
	FabricID    string `json:"fabric_id"`
	FabricModel string `json:"fabric_model"`
	MaxDevice   int    `json:"max_device"`
	MinDevice   int    `json:"min_device"`
}
