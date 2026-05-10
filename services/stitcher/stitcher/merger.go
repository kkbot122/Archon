package stitcher

import (
	"encoding/json"
	"fmt"
	"os"
)

// DeepMergeJSON reads two JSON files, merges their keys (target wins on conflicts), and writes back.
// This is critical for combining multiple package.json or brick.json dependencies safely.
func DeepMergeJSON(sourcePath, targetPath string) error {
	sourceData, err := readJSONMap(sourcePath)
	if err != nil {
		return err
	}

	targetData, err := readJSONMap(targetPath)
	if err != nil {
		// If target doesn't exist yet, just copy source to target
		if os.IsNotExist(err) {
			return copyFile(sourcePath, targetPath)
		}
		return err
	}

	// Merge target into source (target takes precedence)
	merged := mergeMaps(sourceData, targetData)

	mergedBytes, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize merged JSON: %w", err)
	}

	return os.WriteFile(targetPath, mergedBytes, 0644)
}

func readJSONMap(path string) (map[string]interface{}, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return data, nil
}

func mergeMaps(map1, map2 map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range map1 {
		out[k] = v
	}
	for k, v := range map2 {
		if vMap, ok1 := v.(map[string]interface{}); ok1 {
			if existingV, ok2 := out[k].(map[string]interface{}); ok2 {
				out[k] = mergeMaps(existingV, vMap) // recursive deep merge
				continue
			}
		}
		out[k] = v
	}
	return out
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}