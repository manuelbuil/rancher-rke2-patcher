package patcher

import (
	"path/filepath"
	"testing"
)

func TestBuildHelmChartConfig_UsesDataDirEnv(t *testing.T) {
	t.Setenv("RKE2_PATCHER_DATA_DIR", "/tmp/from-data-env")

	filePath, _, _ := BuildHelmChartConfig("rke2-traefik", "rke2-traefik", "rancher/hardened-traefik", "v3.4.0")

	expectedPath := filepath.Join("/tmp/from-data-env", "server", "manifests", "rke2-traefik-config-rke2-patcher.yaml")
	if filePath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, filePath)
	}
}
