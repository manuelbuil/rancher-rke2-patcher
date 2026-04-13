package cmd

type imageListOptions struct {
	WithCVEs bool
	Verbose  bool
}

type imagePatchOptions struct {
	DryRun bool
	Revert bool
}

type cveListEntry struct {
	CVEs  []string
	Error string
}

type patchLimitState struct {
	Entries map[string]patchLimitEntry `json:"entries"`
}

type patchLimitEntry struct {
	Component      string `json:"component"`
	ClusterVersion string `json:"clusterVersion"`
	BaselineTag    string `json:"baselineTag"`
	PatchedToTag   string `json:"patchedToTag"`
}

type patchLimitDecision struct {
	ShouldPersist bool
	StateFilePath string
	EntryKey      string
	Entry         patchLimitEntry
}
