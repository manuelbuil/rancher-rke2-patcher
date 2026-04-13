package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultRKE2DataDir     = "/var/lib/rancher/rke2"
	patchLimitCacheDirEnv  = "RKE2_PATCHER_CACHE_DIR"
	patchLimitStateSubPath = "server/rke2-patcher-cache/patch-limit-state.json"
)

func evaluatePatchLimit(componentName string, currentTag string, targetTag string, revert bool) (patchLimitDecision, error) {
	clusterVersion, err := clusterVersionResolver()
	if err != nil {
		return patchLimitDecision{}, fmt.Errorf("failed to resolve cluster version for patch-limit/revert check: %w", err)
	}

	stateFilePath := patchLimitStateFilePath()
	state, err := loadPatchLimitState(stateFilePath)
	if err != nil {
		return patchLimitDecision{}, err
	}

	entryKey := patchLimitEntryKey(clusterVersion, componentName)
	if revert {
		existing, found := state.Entries[entryKey]
		if !found {
			return patchLimitDecision{}, fmt.Errorf("refusing to revert: component %q has no recorded baseline for RKE2 %s; reverting below the release baseline is not supported", componentName, clusterVersion)
		}

		baselineTag := strings.TrimSpace(existing.BaselineTag)
		if baselineTag == "" {
			return patchLimitDecision{}, fmt.Errorf("refusing to revert: baseline tag is missing for component %q on RKE2 %s", componentName, clusterVersion)
		}

		targetOlderThanBaseline, compareErr := isTagOlderThan(targetTag, baselineTag)
		if compareErr != nil {
			return patchLimitDecision{}, fmt.Errorf("refusing to revert: failed to compare target tag %q with baseline %q: %w", targetTag, baselineTag, compareErr)
		}

		if targetOlderThanBaseline {
			return patchLimitDecision{}, fmt.Errorf("refusing to revert: target tag %q is older than the release baseline %q for component %q on RKE2 %s", targetTag, baselineTag, componentName, clusterVersion)
		}

		return patchLimitDecision{}, nil
	}

	if existing, found := state.Entries[entryKey]; found {
		return patchLimitDecision{}, fmt.Errorf("refusing to patch: component %q was already patched once for RKE2 %s (baseline: %q, patched-to: %q); upgrade RKE2 to patch again", componentName, clusterVersion, existing.BaselineTag, existing.PatchedToTag)
	}

	entry := patchLimitEntry{
		Component:      componentName,
		ClusterVersion: clusterVersion,
		BaselineTag:    currentTag,
		PatchedToTag:   targetTag,
	}

	return patchLimitDecision{
		ShouldPersist: true,
		StateFilePath: stateFilePath,
		EntryKey:      entryKey,
		Entry:         entry,
	}, nil
}

func persistPatchLimitDecision(decision patchLimitDecision) error {
	if !decision.ShouldPersist {
		return nil
	}

	state, err := loadPatchLimitState(decision.StateFilePath)
	if err != nil {
		return err
	}

	if existing, found := state.Entries[decision.EntryKey]; found {
		if existing.PatchedToTag == decision.Entry.PatchedToTag && existing.BaselineTag == decision.Entry.BaselineTag {
			return nil
		}

		return fmt.Errorf("component %q is already recorded as patched once for RKE2 %s", existing.Component, existing.ClusterVersion)
	}

	state.Entries[decision.EntryKey] = decision.Entry
	return savePatchLimitState(decision.StateFilePath, state)
}

func patchLimitStateFilePath() string {
	cacheDir := strings.TrimSpace(os.Getenv(patchLimitCacheDirEnv))
	if cacheDir != "" {
		return filepath.Join(cacheDir, "patch-limit-state.json")
	}

	dataDir := strings.TrimSpace(os.Getenv("RKE2_PATCHER_DATA_DIR"))
	if dataDir == "" {
		dataDir = defaultRKE2DataDir
	}

	return filepath.Join(dataDir, patchLimitStateSubPath)
}

func patchLimitEntryKey(clusterVersion string, componentName string) string {
	return strings.TrimSpace(clusterVersion) + "|" + strings.ToLower(strings.TrimSpace(componentName))
}

func loadPatchLimitState(filePath string) (patchLimitState, error) {
	state := patchLimitState{Entries: map[string]patchLimitEntry{}}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return patchLimitState{}, fmt.Errorf("failed to read patch-limit state file %q: %w", filePath, err)
	}

	if strings.TrimSpace(string(content)) == "" {
		return state, nil
	}

	if err := json.Unmarshal(content, &state); err != nil {
		return patchLimitState{}, fmt.Errorf("failed to parse patch-limit state file %q: %w", filePath, err)
	}

	if state.Entries == nil {
		state.Entries = map[string]patchLimitEntry{}
	}

	return state, nil
}

func savePatchLimitState(filePath string, state patchLimitState) error {
	if state.Entries == nil {
		state.Entries = map[string]patchLimitEntry{}
	}

	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize patch-limit state: %w", err)
	}

	stateDir := filepath.Dir(filePath)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create patch-limit state directory %q: %w", stateDir, err)
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write temporary patch-limit state file %q: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to replace patch-limit state file %q: %w", filePath, err)
	}

	return nil
}

func ensureManifestsDirectoryExists(filePath string) error {
	manifestsDir := strings.TrimSpace(filepath.Dir(filePath))
	if manifestsDir == "" {
		return fmt.Errorf("failed to resolve manifests directory from output path %q; set RKE2_PATCHER_DATA_DIR (for example /var/lib/rancher/rke2)", filePath)
	}

	info, err := os.Stat(manifestsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("manifests directory %q does not exist; set RKE2_PATCHER_DATA_DIR to point to the RKE2 data directory", manifestsDir)
		}
		return fmt.Errorf("failed to verify manifests directory %q: %w", manifestsDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("manifests path %q is not a directory; set RKE2_PATCHER_DATA_DIR to point to the RKE2 data directory", manifestsDir)
	}

	return nil
}
