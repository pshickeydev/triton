//go:build !windows

package doc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSave_PreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "set.tidal")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	d := New()
	if err := d.Save(path, "new\n"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %v, want %v", got, os.FileMode(0o600))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("save left %d entries, want 1 (no temp files)", len(entries))
	}
}

func TestSave_FollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.tidal")
	if err := os.WriteFile(target, []byte("old\n"), filePerm); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.tidal")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	d := New()
	if err := d.Save(link, "new\n"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("save should not replace the symlink itself")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Errorf("target content = %q, want %q", got, "new\n")
	}
}

func TestSave_ReadOnlyTargetFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write read-only files")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "set.tidal")
	if err := os.WriteFile(path, []byte("old\n"), filePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}

	d := New()
	if err := d.Save(path, "new\n"); err == nil {
		t.Fatal("saving over a read-only file should fail")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Errorf("failed save changed the content to %q", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("failed save left %d entries, want 1 (no temp files)", len(entries))
	}
}
