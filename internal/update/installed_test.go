package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteSidecarStamp(t *testing.T) {
	dir := t.TempDir()
	if v := ReadSidecarStamp(dir); v != "" {
		t.Fatalf("empty dir: got %q", v)
	}
	if err := WriteSidecarStamp(dir, "0.2.7"); err != nil {
		t.Fatal(err)
	}
	if got := ReadSidecarStamp(dir); got != "0.2.7" {
		t.Fatalf("got %q", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, sidecarStampFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "0.2.7") {
		t.Fatalf("stamp file: %s", data)
	}
}
