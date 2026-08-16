package appshell

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNeedsRefreshMissingDest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	write(t, src, "x")

	refresh, err := needsRefresh(filepath.Join(dir, "missing-cli"), filepath.Join(dir, "missing-daemon"))
	if err != nil {
		t.Fatalf("needsRefresh: %v", err)
	}
	if !refresh {
		t.Error("missing destinations must trigger refresh")
	}
}

// Regression for the 2026-08-16 incident: rmbd-desktop was overwritten with
// daemon log text carrying a *newer* mtime, so the mtime heuristic alone
// never reinstalled it and the tray was stuck on "Starting…" forever.
func TestNeedsRefreshClobberedDest(t *testing.T) {
	dir := t.TempDir()
	cliDst := filepath.Join(dir, "rmb")
	daemonDst := filepath.Join(dir, "rmbd-desktop")
	write(t, cliDst, machoFake)
	write(t, daemonDst, "2026/08/15 23:27:49 goose: no migrations to run. current version: 9\n")

	// Newer mtime must not protect a non-executable file.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(daemonDst, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	refresh, err := needsRefresh(cliDst, daemonDst)
	if err != nil {
		t.Fatalf("needsRefresh: %v", err)
	}
	if !refresh {
		t.Error("destination without executable magic must trigger refresh even with a newer mtime")
	}
}

func TestIsExecutableImage(t *testing.T) {
	dir := t.TempDir()

	macho := filepath.Join(dir, "macho")
	write(t, macho, machoFake)
	if !isExecutableImage(macho) {
		t.Error("Mach-O 64 magic must be accepted")
	}

	text := filepath.Join(dir, "text")
	write(t, text, "plain log line\n")
	if isExecutableImage(text) {
		t.Error("text file must be rejected")
	}

	elf := filepath.Join(dir, "elf")
	write(t, elf, "\x7fELF\x02\x01\x01\x00")
	if isExecutableImage(elf) {
		t.Error("ELF must be rejected on darwin")
	}

	empty := filepath.Join(dir, "empty")
	write(t, empty, "")
	if isExecutableImage(empty) {
		t.Error("empty file must be rejected")
	}

	if isExecutableImage(filepath.Join(dir, "missing")) {
		t.Error("missing file must be rejected")
	}
}

func TestHasExecutableMagic(t *testing.T) {
	for _, m := range [][4]byte{
		{0xcf, 0xfa, 0xed, 0xfe}, // MH_MAGIC_64 LE
		{0xce, 0xfa, 0xed, 0xfe}, // MH_MAGIC LE
		{0xca, 0xfe, 0xba, 0xbe}, // FAT_MAGIC
		{0xca, 0xfe, 0xba, 0xbf}, // FAT_MAGIC_64
	} {
		if !hasExecutableMagic(m) {
			t.Errorf("magic %x must be accepted on darwin", m)
		}
	}
	for _, m := range [][4]byte{
		{0x32, 0x30, 0x32, 0x36}, // "2026" — clobbered log text
		{0x7f, 'E', 'L', 'F'},    // ELF
		{'M', 'Z', 0x90, 0x00},   // PE
		{'#', '!', '/', 'b'},     // script
	} {
		if hasExecutableMagic(m) {
			t.Errorf("magic %x must be rejected on darwin", m)
		}
	}
}

// machoFake is a minimal MH_MAGIC_64 (little-endian) header prefix.
const machoFake = "\xcf\xfa\xed\xfe\x00\x00\x00\x01fake-mach-o-payload"

func TestIsNewer(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	write(t, src, "x")
	write(t, dst, "x")

	newer, err := isNewer(src, dst)
	if err != nil {
		t.Fatalf("isNewer: %v", err)
	}
	if newer {
		t.Error("same mtime must not be newer")
	}

	// Bump src mtime into the future.
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(src, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	newer, err = isNewer(src, dst)
	if err != nil {
		t.Fatalf("isNewer: %v", err)
	}
	if !newer {
		t.Error("future src mtime must be newer")
	}
}

func TestFindPrefixedBinary(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "rmb-aarch64-apple-darwin"), "x")
	write(t, filepath.Join(dir, "unrelated"), "x")

	if got := findPrefixedBinary(dir, "rmb"); got == "" {
		t.Fatal("prefixed binary not found")
	} else if filepath.Base(got) != "rmb-aarch64-apple-darwin" {
		t.Errorf("found %q", got)
	}

	// Exact name wins.
	write(t, filepath.Join(dir, "rmb"), "x")
	if got := findPrefixedBinary(dir, "rmb"); filepath.Base(got) != "rmb" {
		t.Errorf("exact name should win, got %q", got)
	}

	if got := findPrefixedBinary(dir, "rmbd"); got != "" {
		t.Errorf("nothing matches rmbd, got %q", got)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
