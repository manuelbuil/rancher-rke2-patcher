package patcher

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"dario.cat/mergo"
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
	sigsyaml "sigs.k8s.io/yaml"
)

const (
	registryEnv = "RKE2_PATCHER_REGISTRY"

	defaultNamespace    = "kube-system"
	defaultRegistryHost = "registry.rancher.com"
)

// BuildHelmChartConfigObject generates a typed HelmChartConfig and the generated valuesContent.
func BuildHelmChartConfigObject(componentName string, defaultChartConfigName string, imageName string, imageTag string) (*helmv1.HelmChartConfig, string, error) {
	repo := imageRepositoryWithoutRegistry(imageName)
	valuesContent := strings.TrimSpace(renderValuesContent(componentName, defaultChartConfigName, repo, imageTag))
	name := strings.TrimSpace(defaultChartConfigName)
	if name == "" {
		return nil, "", fmt.Errorf("HelmChartConfig name cannot be empty")
	}

	chart := &helmv1.HelmChartConfig{Spec: helmv1.HelmChartConfigSpec{ValuesContent: valuesContent}}
	chart.Name = name
	chart.Namespace = defaultNamespace
	ensureTypeMeta(chart, nil)

	return chart, valuesContent, nil
}

// MergeHelmChartConfig merges an existing HelmChartConfig content with a generated typed HelmChartConfig.
func MergeHelmChartConfig(generatedChart *helmv1.HelmChartConfig, existingContent string) (*helmv1.HelmChartConfig, error) {
	if generatedChart == nil {
		return nil, fmt.Errorf("generated HelmChartConfig cannot be nil")
	}

	targetName := strings.TrimSpace(generatedChart.Name)
	targetNamespace := strings.TrimSpace(generatedChart.Namespace)
	if targetName == "" || targetNamespace == "" {
		return nil, fmt.Errorf("generated HelmChartConfig is missing metadata.name or metadata.namespace")
	}

	// Start with the generated chart as the base when no existing chart is present.
	mergedChart := generatedChart.DeepCopy()
	mergedChart.Name = targetName
	mergedChart.Namespace = targetNamespace
	ensureTypeMeta(mergedChart, nil)

	trimmedExistingContent := strings.TrimSpace(existingContent)
	if trimmedExistingContent != "" {
		existingChart, err := parseSingleHelmChartConfig(trimmedExistingContent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse existing HelmChartConfig: %w", err)
		}

		name := strings.TrimSpace(existingChart.Name)
		namespace := strings.TrimSpace(existingChart.Namespace)
		if name != targetName || namespace != targetNamespace {
			return nil, fmt.Errorf("existing HelmChartConfig identity mismatch: got %s/%s, want %s/%s", namespace, name, targetNamespace, targetName)
		}

		// Preserve the full existing object shape
		mergedChart = existingChart.DeepCopy()
		mergedChart.Name = targetName
		mergedChart.Namespace = targetNamespace
		ensureTypeMeta(mergedChart, generatedChart)

		// Merge valuesContent while preserving all other existing spec fields unchanged.
		existingValues := existingChart.Spec.ValuesContent
		generatedValues := generatedChart.Spec.ValuesContent

		if existingValues != "" && generatedValues != "" {
			combinedValues, err := mergeValuesContent(existingValues, generatedValues)
			if err != nil {
				return nil, err
			}
			mergedChart.Spec.ValuesContent = combinedValues
		} else if existingValues == "" && generatedValues != "" {
			mergedChart.Spec.ValuesContent = strings.TrimSpace(generatedValues)
		}
	}

	return mergedChart, nil
}

// HelmChartConfigIdentity returns metadata.name and metadata.namespace from a typed HelmChartConfig.
func HelmChartConfigIdentity(chart *helmv1.HelmChartConfig) (string, string, error) {
	if chart == nil {
		return "", "", fmt.Errorf("HelmChartConfig cannot be nil")
	}

	name := strings.TrimSpace(chart.Name)
	namespace := strings.TrimSpace(chart.Namespace)
	if name == "" || namespace == "" {
		return "", "", fmt.Errorf("HelmChartConfig missing metadata.name or metadata.namespace")
	}

	return name, namespace, nil
}

// parseSingleHelmChartConfig generates the HelmChartConfig
func parseSingleHelmChartConfig(content string) (*helmv1.HelmChartConfig, error) {
	decoder := yaml.NewDecoder(strings.NewReader(content))
	for {
		var rawObj map[string]any
		err := decoder.Decode(&rawObj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rawObj) == 0 {
			continue
		}

		// Check if this is a HelmChartConfig document
		kind, ok := rawObj["kind"].(string)
		if !ok || !strings.EqualFold(strings.TrimSpace(kind), "HelmChartConfig") {
			continue
		}

		// Unmarshal into the typed struct using sigs.k8s.io/yaml
		chartBytes, err := sigsyaml.Marshal(rawObj)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal raw object: %w", err)
		}

		chart := &helmv1.HelmChartConfig{}
		if err := sigsyaml.Unmarshal(chartBytes, chart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal HelmChartConfig: %w", err)
		}

		return chart, nil
	}

	return nil, fmt.Errorf("no HelmChartConfig document found")
}

// ParseHelmChartConfig parses content and returns a typed HelmChartConfig.
func ParseHelmChartConfig(content string) (*helmv1.HelmChartConfig, error) {
	return parseSingleHelmChartConfig(content)
}

// MarshalHelmChartConfig serializes a typed HelmChartConfig to YAML.
func MarshalHelmChartConfig(chart *helmv1.HelmChartConfig) (string, error) {
	// Use sigs.k8s.io/yaml for standard k8s-style YAML marshaling
	b, err := sigsyaml.Marshal(chart)
	if err != nil {
		return "", fmt.Errorf("failed to serialize HelmChartConfig: %w", err)
	}

	return string(b), nil
}

func ensureTypeMeta(chart *helmv1.HelmChartConfig, fallback *helmv1.HelmChartConfig) {
	if strings.TrimSpace(chart.APIVersion) == "" {
		if fallback != nil && strings.TrimSpace(fallback.APIVersion) != "" {
			chart.APIVersion = fallback.APIVersion
		} else {
			chart.APIVersion = "helm.cattle.io/v1"
		}
	}
	if strings.TrimSpace(chart.Kind) == "" {
		if fallback != nil && strings.TrimSpace(fallback.Kind) != "" {
			chart.Kind = fallback.Kind
		} else {
			chart.Kind = "HelmChartConfig"
		}
	}
}

func mergeMapsWithOverride(base map[string]any, overlay map[string]any) (map[string]any, error) {
	result := runtime.DeepCopyJSON(base)
	if result == nil {
		result = map[string]any{}
	}

	if overlay != nil {
		if err := mergo.Merge(&result, overlay, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge overlay values: %w", err)
		}
	}

	return result, nil
}

// mergeValuesContent merges the existing and incoming valuesContent blocks, preserving existing keys and overriding with incoming keys where applicable.
func mergeValuesContent(existing string, incoming string) (string, error) {
	existingTrimmed := strings.TrimSpace(existing)
	incomingTrimmed := strings.TrimSpace(incoming)

	if existingTrimmed == "" {
		return incoming, nil
	}
	if incomingTrimmed == "" {
		return existing, nil
	}

	var existingValues any
	if err := yaml.Unmarshal([]byte(existing), &existingValues); err != nil {
		return "", fmt.Errorf("failed to parse existing valuesContent: %w", err)
	}

	var incomingValues any
	if err := yaml.Unmarshal([]byte(incoming), &incomingValues); err != nil {
		return "", fmt.Errorf("failed to parse generated valuesContent: %w", err)
	}

	mergedValues := runtime.DeepCopyJSONValue(incomingValues)
	existingMap, existingIsMap := existingValues.(map[string]any)
	incomingMap, incomingIsMap := incomingValues.(map[string]any)
	if existingIsMap && incomingIsMap {
		mergedMap, err := mergeMapsWithOverride(existingMap, incomingMap)
		if err != nil {
			return "", fmt.Errorf("failed to merge valuesContent maps: %w", err)
		}
		mergedValues = mergedMap
	}

	b, err := yaml.Marshal(mergedValues)
	if err != nil {
		return "", err
	}

	// Indent each line by 4 spaces to match the original style
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for i, line := range lines {
		lines[i] = "    " + line
	}
	return strings.Join(lines, "\n"), nil
}

// dedentYAML removes common leading indentation from YAML content (handles block scalar indentation).
// Special handling: if the first line is a YAML key (no leading spaces, ends with `:`) with content
// following it that has significant indentation, skip the first line when calculating minimum indentation.
// This handles block scalar extraction where the key is at indent 0 but content is indented.
func dedentYAML(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return content
	}

	firstLine := lines[0]
	firstTrimmed := strings.TrimSpace(firstLine)
	firstIndent := len(firstLine) - len(strings.TrimLeft(firstLine, " \t"))

	// Check if first line is a YAML key with no indentation followed by indented content
	skipFirstLine := false
	if firstIndent == 0 && strings.HasSuffix(firstTrimmed, ":") && len(lines) > 1 {
		// Check if there are subsequent non-empty lines with indentation
		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			indent := len(line) - len(strings.TrimLeft(line, " \t"))
			if indent > 0 {
				skipFirstLine = true
				break
			}
		}
	}

	// Find minimum indentation of non-empty lines
	minIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if skipFirstLine && i == 0 {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return content
	}

	// Remove common indentation from all lines
	dedented := make([]string, len(lines))
	for i, line := range lines {
		if len(line) >= minIndent && strings.TrimSpace(line) != "" {
			dedented[i] = line[minIndent:]
		} else if strings.TrimSpace(line) == "" {
			dedented[i] = ""
		} else {
			dedented[i] = line
		}
	}

	return strings.Join(dedented, "\n")
}

func SubtractPatcherValuesContent(existingFileContent, generatedValuesContent string) (*helmv1.HelmChartConfig, error) {
	existingChart, err := parseSingleHelmChartConfig(existingFileContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse existing HelmChartConfig: %w", err)
	}

	existingValuesStr := strings.TrimSpace(existingChart.Spec.ValuesContent)
	if existingValuesStr == "" {
		return existingChart, nil
	}

	trimmedGenerated := strings.TrimSpace(generatedValuesContent)
	if trimmedGenerated == "" {
		return existingChart, nil
	}

	var generatedValues map[string]any
	if err := sigsyaml.Unmarshal([]byte(trimmedGenerated), &generatedValues); err != nil {
		return nil, fmt.Errorf("failed to parse generated valuesContent: %w", err)
	}

	var existingValues map[string]any
	if err := sigsyaml.Unmarshal([]byte(existingValuesStr), &existingValues); err != nil {
		// If normal parsing fails, try with dedenting (handles block scalar extraction)
		dedentedValues := dedentYAML(existingValuesStr)
		if dedentedValues != existingValuesStr {
			if err := sigsyaml.Unmarshal([]byte(dedentedValues), &existingValues); err != nil {
				return nil, fmt.Errorf("failed to parse existing valuesContent: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse existing valuesContent: %w", err)
		}
	}

	// Only update the manifest if we actually removed patcher-managed keys; if nothing was removed (meaning
	// generated values don't exactly match existing keys), return the original manifest unchanged (no-op).
	resultValues := subtractExactMatches(existingValues, generatedValues)
	if reflect.DeepEqual(resultValues, existingValues) {
		return existingChart, nil
	}

	// Update the chart with new values
	if len(resultValues) == 0 {
		existingChart.Spec.ValuesContent = ""
	} else {
		b, err := yaml.Marshal(resultValues)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize updated valuesContent: %w", err)
		}
		existingChart.Spec.ValuesContent = string(b)
	}

	return existingChart, nil
}

// subtractExactMatches removes keys from base where the value exactly matches the corresponding value in toRemove.
// Only top-level exact matches are removed; partial matches (nested structures that differ) are left untouched
// to respect user ownership of modified configurations.
func subtractExactMatches(base, toRemove map[string]any) map[string]any {
	result := map[string]any{}
	if base != nil {
		result = runtime.DeepCopyJSON(base)
	}
	for key, removeValue := range toRemove {
		existingValue, found := result[key]
		if !found {
			continue
		}

		if reflect.DeepEqual(existingValue, removeValue) {
			delete(result, key)
		}
	}
	return result
}

// renderHelmChartConfig generates the content of a HelmChartConfig manifest for the given component, chart, and image details
func renderHelmChartConfig(chartName string, namespace string, valuesContent string) string {
	indentedValuesContent := indentValuesContentBlock(valuesContent)
	return fmt.Sprintf(`apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: %s
  namespace: %s
spec:
  valuesContent: |-
%s
`, chartName, namespace, indentedValuesContent)
}

// indentValuesContentBlock indents each line of the valuesContent block by 4 spaces to match the HelmChartConfig manifest style
func indentValuesContentBlock(valuesContent string) string {
	trimmed := strings.TrimRight(valuesContent, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}

		dedented := line
		if minIndent > 0 && len(line) >= minIndent {
			dedented = line[minIndent:]
		}

		lines[i] = "    " + dedented
	}

	return strings.Join(lines, "\n")
}

// renderValuesContent generates the valuesContent block for the HelmChartConfig based on the component and chart names
func renderValuesContent(componentName string, chartName string, imageName string, imageTag string) string {
	if strings.EqualFold(chartName, "rke2-ingress-nginx") {
		return fmt.Sprintf(`    controller: # change made by rke2-patcher
      image: # change made by rke2-patcher
        repository: %s # change made by rke2-patcher
        primeTag: %s # change made by rke2-patcher`, imageName, imageTag)
	}

	if strings.EqualFold(componentName, "rke2-canal-calico") {
		return fmt.Sprintf("    calico: # change made by rke2-patcher\n"+
			"      cniImage: # change made by rke2-patcher\n"+
			"        repository: %s # change made by rke2-patcher\n"+
			"        tag: %s # change made by rke2-patcher\n"+
			"      nodeImage: # change made by rke2-patcher\n"+
			"        repository: %s # change made by rke2-patcher\n"+
			"        tag: %s # change made by rke2-patcher\n"+
			"      flexvolImage: # change made by rke2-patcher\n"+
			"        repository: %s # change made by rke2-patcher\n"+
			"        tag: %s # change made by rke2-patcher\n"+
			"      kubeControllerImage: # change made by rke2-patcher\n"+
			"        repository: %s # change made by rke2-patcher\n"+
			"        tag: %s # change made by rke2-patcher",
			imageName, imageTag, imageName, imageTag, imageName, imageTag, imageName, imageTag)
	}

	if strings.EqualFold(componentName, "rke2-canal-flannel") {
		return fmt.Sprintf(`    flannel: # change made by rke2-patcher
      image: # change made by rke2-patcher
        repository: %s # change made by rke2-patcher
        tag: %s # change made by rke2-patcher`, imageName, imageTag)
	}

	if strings.EqualFold(componentName, "rke2-flannel") {
		return fmt.Sprintf(`    flannel:
      image:
        repository: %s
        tag: %s`, imageName, imageTag)
	}

	if strings.EqualFold(componentName, "rke2-coredns-cluster-autoscaler") {
		return fmt.Sprintf(`    autoscaler: # change made by rke2-patcher
      image: # change made by rke2-patcher
        repository: %s # change made by rke2-patcher
        tag: %s # change made by rke2-patcher`, imageName, imageTag)
	}

	return fmt.Sprintf(`    image: # change made by rke2-patcher
      repository: %s # change made by rke2-patcher
      tag: %s # change made by rke2-patcher`, imageName, imageTag)
}

func imageRepositoryWithoutRegistry(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) < 2 {
		return imageName
	}

	first := strings.ToLower(parts[0])
	hasRegistryPrefix := strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost"
	if !hasRegistryPrefix {
		return imageName
	}

	return strings.Join(parts[1:], "/")
}
