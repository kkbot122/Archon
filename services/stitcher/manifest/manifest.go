package manifest

import "encoding/json"

// Metadata maps to the protobuf Metadata message
type Metadata struct {
	ProjectName   string `json:"projectName"`
	TargetCloud   string `json:"targetCloud"`
	SchemaVersion string `json:"schemaVersion"`
}

// Node maps to the protobuf ArchitectureNode message
type Node struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Version string            `json:"version"`
	// Protobuf Structs seamlessly unmarshal into Go map[string]interface{} or map[string]string
	Config  map[string]string `json:"config"` 
}

// Connection maps to the protobuf Connection message
type Connection struct {
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
	Protocol string `json:"protocol"`
}

// Manifest is the root structure
type Manifest struct {
	Metadata    Metadata     `json:"metadata"`
	Nodes       []Node       `json:"nodes"`
	Connections []Connection `json:"connections"`
}

// ParseManifest takes the raw JSON string from Kafka and returns a typed Manifest
func ParseManifest(raw string) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	return &m, nil
}