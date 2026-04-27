package kube

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Extracts the value of global.prime.enabled from HelmChartConfig YAML content
func ExtractPrimeEnabledFromHelmChartConfig(content string) (bool, error) {
	type setBlock struct {
		Set map[string]string `yaml:"set"`
	}
	type hcc struct {
		Spec setBlock `yaml:"spec"`
	}

	var doc hcc
	err := yaml.Unmarshal([]byte(content), &doc)
	if err != nil {
		return false, fmt.Errorf("failed to parse HelmChartConfig YAML: %w", err)
	}

	if doc.Spec.Set == nil {
		return false, fmt.Errorf("no set block found in HelmChartConfig")
	}

	val, ok := doc.Spec.Set["global.prime.enabled"]
	if !ok {
		return false, fmt.Errorf("global.prime.enabled not found in HelmChartConfig")
	}

	if val == "true" {
		return true, nil
	}
	return false, nil
}
