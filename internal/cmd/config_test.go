package cmd

import (
	"testing"
)

func TestCollectConfigEntries_Defaults(t *testing.T) {
	t.Setenv(registryEnvName, "")
	t.Setenv(cveModeEnvName, "")
	t.Setenv(cveNamespaceEnvName, "")
	t.Setenv(cveScannerImageEnvName, "")
	t.Setenv(cveJobTimeoutEnvName, "")
	t.Setenv(dataDirEnvName, "")
	t.Setenv(helmNamespaceEnvName, "")

	entries, err := collectConfigEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry := configEntryByKey(entries, "registry")
	if registry.Effective != "https://registry.rancher.com" {
		t.Fatalf("unexpected default registry effective value: %q", registry.Effective)
	}
	if registry.Source != "default" {
		t.Fatalf("unexpected default registry source: %q", registry.Source)
	}

	scannerMode := configEntryByKey(entries, "scanner_mode")
	if scannerMode.Effective != "cluster" {
		t.Fatalf("unexpected default scanner mode: %q", scannerMode.Effective)
	}

	stateNamespace := configEntryByKey(entries, "rke2_patcher_state_namespace")
	if stateNamespace.Effective != "rke2-patcher" {
		t.Fatalf("unexpected default patch-limit namespace: %q", stateNamespace.Effective)
	}
}

func TestCollectConfigEntries_Overrides(t *testing.T) {
	t.Setenv(registryEnvName, "mirror.local:5000")
	t.Setenv(cveModeEnvName, "local")
	t.Setenv(cveNamespaceEnvName, "sec-scan")
	t.Setenv(cveScannerImageEnvName, "scanner:1.2.3")
	t.Setenv(cveJobTimeoutEnvName, "11m")
	t.Setenv(dataDirEnvName, "/tmp/rke2")
	t.Setenv(helmNamespaceEnvName, "custom-ns")

	entries, err := collectConfigEntries()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	registry := configEntryByKey(entries, "registry")
	if registry.Effective != "https://mirror.local:5000" {
		t.Fatalf("unexpected overridden registry effective value: %q", registry.Effective)
	}
	if registry.Source != registryEnvName {
		t.Fatalf("unexpected registry source: %q", registry.Source)
	}

	scannerMode := configEntryByKey(entries, "scanner_mode")
	if scannerMode.Effective != "local" {
		t.Fatalf("unexpected scanner mode effective value: %q", scannerMode.Effective)
	}
	if scannerMode.Source != cveModeEnvName {
		t.Fatalf("unexpected scanner mode source: %q", scannerMode.Source)
	}

	stateNamespace := configEntryByKey(entries, "rke2_patcher_state_namespace")
	if stateNamespace.Effective != "sec-scan" {
		t.Fatalf("unexpected overridden patch-limit namespace: %q", stateNamespace.Effective)
	}
}

func TestCollectConfigEntries_InvalidValues(t *testing.T) {
	t.Setenv(cveModeEnvName, "invalid")
	if _, err := collectConfigEntries(); err == nil {
		t.Fatalf("expected scanner mode validation error, got nil")
	}

	t.Setenv(cveModeEnvName, "cluster")
	t.Setenv(cveJobTimeoutEnvName, "0")
	if _, err := collectConfigEntries(); err == nil {
		t.Fatalf("expected timeout validation error, got nil")
	}
}

func configEntryByKey(entries []configEntry, key string) configEntry {
	for _, entry := range entries {
		if entry.Key == key {
			return entry
		}
	}
	return configEntry{}
}
