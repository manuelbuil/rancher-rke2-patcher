package cve

import (
	"errors"
	"reflect"
	"testing"
)

func TestListForImagesWithMode_LocalModeFromEnvUsesLocalScanner(t *testing.T) {
	t.Setenv(cveModeEnv, "local")

	originalClusterScanner := scanImagesWithTrivyJob
	originalSingleScanner := listForImageWithMode
	t.Cleanup(func() {
		scanImagesWithTrivyJob = originalClusterScanner
		listForImageWithMode = originalSingleScanner
	})

	clusterCalled := false
	scanImagesWithTrivyJob = func(_ []string, _ bool) ([]byte, error) {
		clusterCalled = true
		return nil, errors.New("cluster scanner should not be called in local mode")
	}

	listForImageWithMode = func(image string, modeOverride string) (Result, error) {
		if modeOverride != "local" {
			t.Fatalf("expected local mode override, got %q", modeOverride)
		}

		switch image {
		case "img-ok":
			return Result{Tool: "trivy", CVEs: []string{"CVE-1"}}, nil
		case "img-fail":
			return Result{}, errors.New("scan failed")
		default:
			return Result{}, errors.New("unexpected image")
		}
	}

	results, errorsByImage, err := ListForImagesWithMode([]string{"img-ok", "img-fail"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if clusterCalled {
		t.Fatalf("cluster scanner was called in local mode")
	}

	expectedResults := map[string]Result{
		"img-ok": {Tool: "trivy", CVEs: []string{"CVE-1"}},
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

func TestListForImagesWithMode_ClusterModeUsesBatchScanner(t *testing.T) {
	originalClusterScanner := scanImagesWithTrivyJob
	originalSingleScanner := listForImageWithMode
	t.Cleanup(func() {
		scanImagesWithTrivyJob = originalClusterScanner
		listForImageWithMode = originalSingleScanner
	})

	localCalled := false
	listForImageWithMode = func(_ string, _ string) (Result, error) {
		localCalled = true
		return Result{}, errors.New("local scanner should not be called in cluster mode")
	}

	scanImagesWithTrivyJob = func(images []string, _ bool) ([]byte, error) {
		expected := []string{"img-a"}
		if !reflect.DeepEqual(images, expected) {
			t.Fatalf("unexpected image batch: %#v", images)
		}

		return []byte("__RKE2_PATCHER_TRIVY_BEGIN__img-a\n{\"Results\":[{\"Vulnerabilities\":[{\"VulnerabilityID\":\"CVE-A\"}]}]}\n__RKE2_PATCHER_TRIVY_RC__img-a__0\n__RKE2_PATCHER_TRIVY_END__img-a\n"), nil
	}

	results, errorsByImage, err := ListForImagesWithMode([]string{"img-a"}, "cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if localCalled {
		t.Fatalf("local scanner was called in cluster mode")
	}

	expectedResults := map[string]Result{
		"img-a": {Tool: "trivy-job-batch", CVEs: []string{"CVE-A"}},
	}
	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("unexpected results: %#v", results)
	}

	if len(errorsByImage) != 0 {
		t.Fatalf("expected no per-image errors, got %#v", errorsByImage)
	}
}
