package cve

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDownloadVEXFileOnce(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`{"@context":"https://openvex.dev/ns/v0.2.0"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{name: "valid archive", content: compressed.Bytes()},
		{name: "invalid archive", content: []byte("not gzip"), wantErr: true},
		{name: "truncated archive", content: compressed.Bytes()[:compressed.Len()-4], wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(tt.content)
			}))
			defer server.Close()

			dir := t.TempDir()
			path := filepath.Join(dir, vexFileName)
			if err := os.WriteFile(path, []byte("cached report"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := downloadVEXFileOnce(server.URL, dir, path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("downloadVEXFileOnce() error = %v, wantErr %v", err, tt.wantErr)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := []byte(`{"@context":"https://openvex.dev/ns/v0.2.0"}`)
			if tt.wantErr {
				want = []byte("cached report")
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("cached report = %q, want %q", got, want)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("unexpected temporary files after download: %v", entries)
			}
		})
	}
}

func TestListForImages_LocalModeFromEnvUsesLocalScanner(t *testing.T) {
	t.Setenv(scannerModeEnv, "local")

	originalClusterScanner := scanImagesWithTrivyJob
	originalSingleScanner := listCVEsForImageLocal
	t.Cleanup(func() {
		scanImagesWithTrivyJob = originalClusterScanner
		listCVEsForImageLocal = originalSingleScanner
	})

	clusterCalled := false
	scanImagesWithTrivyJob = func(_ []string) ([]byte, error) {
		clusterCalled = true
		return nil, errors.New("cluster scanner should not be called in local mode")
	}

	listCVEsForImageLocal = func(image string) (ResultCVEs, error) {
		switch image {
		case "img-ok":
			return ResultCVEs{Tool: "trivy", CVEs: []Vulnerability{{ID: "CVE-1", Severity: "HIGH"}}}, nil
		case "img-fail":
			return ResultCVEs{}, errors.New("scan failed")
		default:
			return ResultCVEs{}, errors.New("unexpected image")
		}
	}

	results, errorsByImage, err := ListCVEsForImages([]string{"img-ok", "img-fail"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if clusterCalled {
		t.Fatalf("cluster scanner was called in local mode")
	}

	expectedResults := map[string]ResultCVEs{
		"img-ok": {Tool: "trivy", CVEs: []Vulnerability{{ID: "CVE-1", Severity: "HIGH"}}},
	}
	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("unexpected results: %#v", results)
	}

	if len(errorsByImage) != 1 {
		t.Fatalf("expected one per-image error, got %d", len(errorsByImage))
	}
	if scanErr, found := errorsByImage["img-fail"]; !found || scanErr == nil || scanErr.Error() != "scan failed" {
		t.Fatalf("unexpected per-image error map: %#v", errorsByImage)
	}
}

func TestListForImages_ClusterModeUsesBatchScanner(t *testing.T) {
	originalClusterScanner := scanImagesWithTrivyJob
	originalSingleScanner := listCVEsForImageLocal
	t.Cleanup(func() {
		scanImagesWithTrivyJob = originalClusterScanner
		listCVEsForImageLocal = originalSingleScanner
	})

	localCalled := false
	listCVEsForImageLocal = func(_ string) (ResultCVEs, error) {
		localCalled = true
		return ResultCVEs{}, errors.New("local scanner should not be called in cluster mode")
	}

	scanImagesWithTrivyJob = func(images []string) ([]byte, error) {
		expected := []string{"img-a"}
		if !reflect.DeepEqual(images, expected) {
			t.Fatalf("unexpected image batch: %#v", images)
		}

		return []byte("__RKE2_PATCHER_TRIVY_BEGIN__img-a\n{\"Results\":[{\"Vulnerabilities\":[{\"VulnerabilityID\":\"CVE-A\",\"Severity\":\"HIGH\"}]}]}\n__RKE2_PATCHER_TRIVY_RC__img-a__0\n__RKE2_PATCHER_TRIVY_END__img-a\n"), nil
	}

	results, errorsByImage, err := ListCVEsForImages([]string{"img-a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if localCalled {
		t.Fatalf("local scanner was called in cluster mode")
	}

	expectedResults := map[string]ResultCVEs{
		"img-a": {Tool: "trivy-job-batch", CVEs: []Vulnerability{{ID: "CVE-A", Severity: "HIGH"}}},
	}
	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("unexpected results: %#v", results)
	}

	if len(errorsByImage) != 0 {
		t.Fatalf("expected no per-image errors, got %#v", errorsByImage)
	}
}

func TestTrivyCVEsFromJSONPreservesSeverityAndDeduplicatesToHighest(t *testing.T) {
	output := []byte(`{"Results":[{"Vulnerabilities":[{"VulnerabilityID":"CVE-A","Severity":"HIGH"},{"VulnerabilityID":"CVE-B","Severity":"CRITICAL"},{"VulnerabilityID":"CVE-A","Severity":"CRITICAL"},{"VulnerabilityID":"CVE-C","Severity":"MEDIUM"},{"VulnerabilityID":"CVE-0","Severity":"HIGH"}]}]}`)

	got, err := trivyCVEsFromJSON(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []Vulnerability{
		{ID: "CVE-A", Severity: "CRITICAL"},
		{ID: "CVE-B", Severity: "CRITICAL"},
		{ID: "CVE-0", Severity: "HIGH"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected vulnerabilities: %#v", got)
	}
}
