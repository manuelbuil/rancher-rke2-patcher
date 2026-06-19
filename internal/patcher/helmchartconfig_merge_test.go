package patcher

import (
	"strings"
	"testing"
)

func TestMergeHelmChartConfigWithContents(t *testing.T) {
	existingContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-traefik
  namespace: kube-system
spec:
  valuesContent: |-
    service:
      type: ClusterIP
`

	generatedContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-traefik
  namespace: kube-system
spec:
  valuesContent: |-
    image:
      repository: rancher/hardened-traefik
      tag: new-tag
`

	generatedChart, err := parseSingleHelmChartConfig(generatedContent)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	mergedChart, err := MergeHelmChartConfig(generatedChart, existingContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged, err := MarshalHelmChartConfig(mergedChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(merged, "type: ClusterIP") {
		t.Fatalf("expected merged content to include existing values content: %s", merged)
	}
	if !strings.Contains(merged, "repository: rancher/hardened-traefik") || !strings.Contains(merged, "tag: new-tag") {
		t.Fatalf("expected merged content to include generated image values: %s", merged)
	}
}

func TestHelmChartConfigIdentityFromContent(t *testing.T) {
	content := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-traefik
  namespace: kube-system
spec:
  valuesContent: |-
    image:
      repository: rancher/hardened-traefik
      tag: v3.4.0
`

	chart, err := parseSingleHelmChartConfig(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	name, namespace, err := HelmChartConfigIdentity(chart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if name != "rke2-traefik" || namespace != "kube-system" {
		t.Fatalf("unexpected identity: %s/%s", namespace, name)
	}
}

func TestMergeHelmChartConfigWithContents_IndentationPreserved(t *testing.T) {

	existingContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
    name: rke2-traefik
    namespace: kube-system
spec:
    valuesContent: |-
        providers:
            kubernetesGateway:
                enabled: true
`

	generatedContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
    name: rke2-traefik
    namespace: kube-system
spec:
    valuesContent: |-
        image: # change made by rke2-patcher
            repository: rancher/hardened-traefik # change made by rke2-patcher
            tag: v3.6.12-build20260409 # change made by rke2-patcher
`

	generatedChart, err := parseSingleHelmChartConfig(generatedContent)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	mergedChart, err := MergeHelmChartConfig(generatedChart, existingContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged, err := MarshalHelmChartConfig(mergedChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that both blocks are present and properly indented (4 spaces)
	wantImageBlock := "    image:"
	wantProvidersBlock := "    providers:"
	wantKubeGatewayBlock := "      kubernetesGateway:"
	wantEnabledBlock := "        enabled: true"

	if !strings.Contains(merged, wantImageBlock) {
		t.Errorf("expected merged content to contain indented image block: %q\nMerged:\n%s", wantImageBlock, merged)
	}
	if !strings.Contains(merged, wantProvidersBlock) {
		t.Errorf("expected merged content to contain indented providers block: %q\nMerged:\n%s", wantProvidersBlock, merged)
	}
	if !strings.Contains(merged, wantKubeGatewayBlock) {
		t.Errorf("expected merged content to contain indented kubernetesGateway block: %q\nMerged:\n%s", wantKubeGatewayBlock, merged)
	}
	if !strings.Contains(merged, wantEnabledBlock) {
		t.Errorf("expected merged content to contain indented enabled block: %q\nMerged:\n%s", wantEnabledBlock, merged)
	}

	// Check that there are no top-level keys without indentation (should not start at column 0)
	lines := strings.Split(merged, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "image:") || strings.HasPrefix(line, "providers:") {
			t.Errorf("found unindented top-level key in merged valuesContent: %q\nMerged:\n%s", line, merged)
		}
	}
}

func TestMergeHelmChartConfigWithContents_AfterReconcileEmptyValuesContent_ProducesIndentedValuesBlock(t *testing.T) {
	generatedChart, _, err := BuildHelmChartConfigObject(
		"rke2-coredns",
		"rke2-coredns",
		"registry.rancher.com/rancher/hardened-coredns",
		"v1.14.3-build20260604",
	)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	existingAfterReconcile := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
spec:
  failurePolicy: reinstall
  valuesContent: ""
`

	mergedAfterReconcileChart, err := MergeHelmChartConfig(generatedChart, existingAfterReconcile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged, err := MarshalHelmChartConfig(mergedAfterReconcileChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(merged, "\nimage: # change made by rke2-patcher") {
		t.Fatalf("expected image key to remain indented under valuesContent, got:\n%s", merged)
	}

	if !strings.Contains(merged, "valuesContent: |-") || !strings.Contains(merged, "image: # change made by rke2-patcher") {
		t.Fatalf("expected valuesContent block to include patcher image key, got:\n%s", merged)
	}

	if !strings.Contains(merged, "failurePolicy: reinstall") {
		t.Fatalf("expected existing non-values spec fields to be preserved, got:\n%s", merged)
	}

}

func TestMergeHelmChartConfigWithContents_PreservesExistingMetadataFields(t *testing.T) {
	existingContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
  labels:
    team: networking
  annotations:
    owner: platform
spec:
  valuesContent: |-
    service:
      type: ClusterIP
`

	generatedChart, _, err := BuildHelmChartConfigObject(
		"rke2-coredns",
		"rke2-coredns",
		"registry.rancher.com/rancher/hardened-coredns",
		"v1.14.3-build20260604",
	)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	mergedChart, err := MergeHelmChartConfig(generatedChart, existingContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged, err := MarshalHelmChartConfig(mergedChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(merged, "team: networking") || !strings.Contains(merged, "owner: platform") {
		t.Fatalf("expected existing metadata labels/annotations to be preserved, got:\n%s", merged)
	}

	if !strings.Contains(merged, "repository: rancher/hardened-coredns") {
		t.Fatalf("expected patcher image values to be applied, got:\n%s", merged)
	}
}

func TestMergeHelmChartConfigWithContents_PreservesExistingSpecFields(t *testing.T) {
	existingContent := `apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns
  namespace: kube-system
spec:
  values:
    featureFlags:
      canary: true
  valuesSecrets:
    - name: coredns-values
      keys:
        - values.yaml
      ignoreUpdates: true
  failurePolicy: reinstall
  valuesContent: |-
    service:
      type: ClusterIP
`

	generatedChart, _, err := BuildHelmChartConfigObject(
		"rke2-coredns",
		"rke2-coredns",
		"registry.rancher.com/rancher/hardened-coredns",
		"v1.14.3-build20260604",
	)
	if err != nil {
		t.Fatalf("unexpected build error: %v", err)
	}

	mergedChart, err := MergeHelmChartConfig(generatedChart, existingContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged, err := MarshalHelmChartConfig(mergedChart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(merged, "valuesSecrets:") || !strings.Contains(merged, "name: coredns-values") {
		t.Fatalf("expected existing spec.valuesSecrets to be preserved, got:\n%s", merged)
	}
	if !strings.Contains(merged, "failurePolicy: reinstall") {
		t.Fatalf("expected existing spec.failurePolicy to be preserved, got:\n%s", merged)
	}
	if !strings.Contains(merged, "repository: rancher/hardened-coredns") {
		t.Fatalf("expected generated image values to be merged into valuesContent, got:\n%s", merged)
	}
}
