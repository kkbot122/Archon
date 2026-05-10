package library

// Index represents the root atomic-library/index.json
type Index struct {
	Version          string      `json:"version"`
	AvailableNodes   []BrickMeta `json:"available_nodes"`
	AllowedProtocols []string    `json:"allowed_protocols"`
}

// BrickMeta represents the metadata for a single template brick
type BrickMeta struct {
	Type            string   `json:"type"`
	Description     string   `json:"description"`
	AllowedVersions []string `json:"allowed_versions"`
	RequiredConfig  []string `json:"required_config"`
	Path            string   `json:"path"`
	BuildCommand    []string `json:"build_command"`
}