package patcher

import (
	"strings"
	"testing"
)

const patcherComment = "# change made by rke2-patcher"

func TestRenderValuesContent_AddsPatcherCommentToImageAndRepositoryLines(t *testing.T) {
	valuesForIngress := renderValuesContent("rke2-ingress-nginx", "rke2-ingress-nginx", "rancher/hardened-ingress-nginx", "v1.0.0")
	if !strings.Contains(valuesForIngress, "repository: rancher/hardened-ingress-nginx # change made by rke2-patcher") {
		t.Fatalf("expected ingress repository line to include patcher comment, got:\n%s", valuesForIngress)
	}
	if !strings.Contains(valuesForIngress, "primeTag: v1.0.0 # change made by rke2-patcher") {
		t.Fatalf("expected ingress tag line to include patcher comment, got:\n%s", valuesForIngress)
	}
}

func TestRenderValuesContent_AllGeneratedLinesHavePatcherComment(t *testing.T) {
	tests := []struct {
		name          string
		componentName string
		chartName     string
		imageName     string
		imageTag      string
	}{
		{name: "default", componentName: "rke2-traefik", chartName: "rke2-traefik", imageName: "rancher/hardened-traefik", imageTag: "v3.6.9"},
		{name: "ingress nginx", componentName: "rke2-ingress-nginx", chartName: "rke2-ingress-nginx", imageName: "rancher/hardened-ingress-nginx", imageTag: "v1.0.0"},
		{name: "canal calico", componentName: "rke2-canal-calico", chartName: "rke2-canal", imageName: "rancher/hardened-calico", imageTag: "v1.0.0"},
		{name: "canal flannel", componentName: "rke2-canal-flannel", chartName: "rke2-canal", imageName: "rancher/hardened-flannel", imageTag: "v1.0.0"},
		{name: "coredns autoscaler", componentName: "rke2-coredns-cluster-autoscaler", chartName: "rke2-coredns", imageName: "rancher/hardened-cluster-autoscaler", imageTag: "v1.10.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := renderValuesContent(tt.componentName, tt.chartName, tt.imageName, tt.imageTag)
			for _, line := range strings.Split(values, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if !strings.Contains(line, patcherComment) {
					t.Fatalf("expected every generated line to include patcher comment, missing on line %q\nfull values:\n%s", line, values)
				}
			}
		})
	}
}

func TestBuildHelmChartConfigWithDataDir_GeneratedContentParsesForPatchedComponents(t *testing.T) {
	tests := []struct {
		name          string
		componentName string
		chartName     string
		imageName     string
		imageTag      string
	}{
		{name: "default", componentName: "rke2-traefik", chartName: "rke2-traefik", imageName: "rancher/hardened-traefik", imageTag: "v3.6.9"},
		{name: "ingress nginx", componentName: "rke2-ingress-nginx", chartName: "rke2-ingress-nginx", imageName: "rancher/hardened-ingress-nginx", imageTag: "v1.0.0"},
		{name: "canal calico", componentName: "rke2-canal-calico", chartName: "rke2-canal", imageName: "rancher/hardened-calico", imageTag: "v1.0.0"},
		{name: "canal flannel", componentName: "rke2-canal-flannel", chartName: "rke2-canal", imageName: "rancher/hardened-flannel", imageTag: "v1.0.0"},
		{name: "coredns autoscaler", componentName: "rke2-coredns-cluster-autoscaler", chartName: "rke2-coredns", imageName: "rancher/hardened-cluster-autoscaler", imageTag: "v1.10.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, valuesContent, err := BuildHelmChartConfigObject(tt.componentName, tt.chartName, tt.imageName, tt.imageTag)
			if err != nil {
				t.Fatalf("unexpected build error: %v", err)
			}

			if strings.TrimSpace(valuesContent) == "" {
				t.Fatal("expected non-empty valuesContent, got empty content")
			}
		})
	}
}

func TestRenderValuesContent_CanalCalicoPatchesFourCalicoImageKeys(t *testing.T) {
	values := renderValuesContent("rke2-canal-calico", "rke2-canal", "rancher/hardened-calico", "v3.31.4")

	if !strings.Contains(values, "cniImage:") || !strings.Contains(values, "nodeImage:") || !strings.Contains(values, "flexvolImage:") || !strings.Contains(values, "kubeControllerImage:") {
		t.Fatalf("expected canal-calico values to patch cni/node/flexvol/kubeController images, got:\n%s", values)
	}
}

func TestRenderValuesContent_CoreDNSClusterAutoscalerUsesAutoscalerImageKeys(t *testing.T) {
	values := renderValuesContent("rke2-coredns-cluster-autoscaler", "rke2-coredns", "rancher/hardened-cluster-autoscaler", "v1.10.3")

	if !strings.Contains(values, "autoscaler:") || !strings.Contains(values, "image:") {
		t.Fatalf("expected coredns-cluster-autoscaler values to patch autoscaler.image keys, got:\n%s", values)
	}

	if !strings.Contains(values, "repository: rancher/hardened-cluster-autoscaler") || !strings.Contains(values, "tag: v1.10.3") {
		t.Fatalf("expected coredns-cluster-autoscaler values to include repository/tag override, got:\n%s", values)
	}
}

func TestRenderHelmChartConfig_IndentsUnindentedValuesContent(t *testing.T) {
	values := "image:\n  repository: rancher/hardened-coredns\n  tag: v1.14.3-build20260604"
	rendered := renderHelmChartConfig("rke2-coredns", "kube-system", values)

	if strings.Contains(rendered, "\nimage:") {
		t.Fatalf("expected valuesContent keys to be indented under block scalar, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "\n    image:\n      repository: rancher/hardened-coredns\n      tag: v1.14.3-build20260604") {
		t.Fatalf("expected valuesContent block to be indented with 4 spaces, got:\n%s", rendered)
	}
}

func TestRenderHelmChartConfig_PreservesAlreadyIndentedValuesContent(t *testing.T) {
	values := "    image:\n      repository: rancher/hardened-coredns\n      tag: v1.14.3-build20260604"
	rendered := renderHelmChartConfig("rke2-coredns", "kube-system", values)

	if strings.Contains(rendered, "\n        image:") {
		t.Fatalf("expected block indentation to remain stable without double-indenting, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "\n    image:\n      repository: rancher/hardened-coredns\n      tag: v1.14.3-build20260604") {
		t.Fatalf("expected stable valuesContent indentation, got:\n%s", rendered)
	}
}
