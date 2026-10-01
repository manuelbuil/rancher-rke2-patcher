package kube

import (
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrivyVEXDownloadScript(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`{"statements":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		name    string
		archive []byte
		wantErr bool
	}{
		{name: "valid archive", archive: compressed.Bytes()},
		{name: "invalid archive", archive: []byte("not gzip"), wantErr: true},
		{name: "truncated archive", archive: compressed.Bytes()[:compressed.Len()-4], wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "source.gz")
			if err := os.WriteFile(archive, tt.archive, 0o600); err != nil {
				t.Fatal(err)
			}
			for name, content := range map[string]string{
				"curl":  "#!/bin/sh\ncp \"$VEX_TEST_ARCHIVE\" \"$4\"\n",
				"sleep": "#!/bin/sh\nexit 0\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}

			dest := filepath.Join(dir, "rancher.openvex.json")
			lines := append([]string(nil), trivyVEXDownloadScriptLines...)
			lines[0] = "VEX_FILE=" + "'" + dest + "'"
			cmd := exec.Command("sh", "-c", strings.Join(lines, "\n"))
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "VEX_TEST_ARCHIVE="+archive)
			output, err := cmd.CombinedOutput()
			if (err != nil) != tt.wantErr {
				t.Fatalf("script error = %v, wantErr %v, output: %s", err, tt.wantErr, output)
			}
			if tt.wantErr {
				if _, err := os.Stat(dest); !os.IsNotExist(err) {
					t.Fatalf("failed download left a JSON file: %v", err)
				}
			} else {
				got, err := os.ReadFile(dest)
				if err != nil || string(got) != `{"statements":[]}` {
					t.Fatalf("decompressed JSON = %q, error = %v", got, err)
				}
			}
			for _, path := range []string{dest + ".gz", dest + ".tmp"} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("download left a temporary file at %s: %v", path, err)
				}
			}
		})
	}
}
