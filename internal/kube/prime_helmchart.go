package kube

import (
	"encoding/json"
	"fmt"
)

// Extracts the value of global.prime.enabled from HelmChart JSON content
func ExtractPrimeEnabledFromHelmChart(content string) (bool, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(content), &obj); err != nil {
		return false, fmt.Errorf("failed to parse HelmChart JSON: %w", err)
	}

	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("no spec block found in HelmChart")
	}

	set, ok := spec["set"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("no set block found in HelmChart")
	}

	val, ok := set["global.prime.enabled"]
	if !ok {
		return false, fmt.Errorf("global.prime.enabled not found in HelmChart")
	}

	switch v := val.(type) {
	case bool:
		return v, nil
	case string:
		return v == "true", nil
	default:
		return false, fmt.Errorf("global.prime.enabled has unexpected type in HelmChart")
	}
}
