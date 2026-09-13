package doc

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	d := New()

	_, err := d.Load(filepath.Join(t.TempDir(), "missing.tidal"))
	if err == nil {
		t.Fatal("loading a missing file should error")
	}
	if d.Path() != "" {
		t.Errorf("failed load should not bind a path, got %q", d.Path())
	}
}

func TestLoad_BindsPathAndContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	want := "d1 $ sound \"bd sn\"\n"
	if err := os.WriteFile(path, []byte(want), filePerm); err != nil {
		t.Fatal(err)
	}

	d := New()
	got, err := d.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("Load() = %q, want %q", got, want)
	}
	if d.Path() != path {
		t.Errorf("Path() = %q, want %q", d.Path(), path)
	}
	if d.Dirty(want) {
		t.Error("buffer matching the loaded content should not be dirty")
	}
	if !d.Dirty("other") {
		t.Error("buffer differing from the loaded content should be dirty")
	}
}

func TestLoad_NormalizesLineEndingsAndTabs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	if err := os.WriteFile(path, []byte("d1 $ sound \"bd\"\r\nd2 $ sound \"sn\"\r\n\td1 $ sound \"hh\"\n"), filePerm); err != nil {
		t.Fatal(err)
	}

	d := New()
	content, err := d.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "d1 $ sound \"bd\"\nd2 $ sound \"sn\"\n    d1 $ sound \"hh\"\n"
	if content != want {
		t.Errorf("Load() = %q, want %q", content, want)
	}
	if d.Dirty(content) {
		t.Error("buffer matching the normalized content should not be dirty")
	}
}

func TestLoad_DropsControlCharactersLikeTheBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	raw := "d1 $ sound \"bd\"\x0c\xff\n"
	if err := os.WriteFile(path, []byte(raw), filePerm); err != nil {
		t.Fatal(err)
	}

	d := New()
	content, err := d.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "d1 $ sound \"bd\"\n"
	if content != want {
		t.Errorf("Load() = %q, want %q", content, want)
	}
	if d.Dirty(content) {
		t.Error("buffer matching the sanitized content should not be dirty")
	}
}

func TestLoad_FailureKeepsBoundDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	if err := os.WriteFile(path, []byte("kept\n"), filePerm); err != nil {
		t.Fatal(err)
	}
	d := New()
	if _, err := d.Load(path); err != nil {
		t.Fatal(err)
	}

	if _, err := d.Load(filepath.Join(t.TempDir(), "missing.tidal")); err == nil {
		t.Fatal("loading a missing file should error")
	}
	if d.Path() != path {
		t.Errorf("failed load changed the path to %q", d.Path())
	}
	if d.Content() != "kept\n" {
		t.Errorf("failed load changed the content to %q", d.Content())
	}
}

func TestExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	if err := os.WriteFile(path, []byte("x"), filePerm); err != nil {
		t.Fatal(err)
	}

	d := New()
	if ok, err := d.Exists(path); err != nil || !ok {
		t.Errorf("Exists(existing) = %v, %v; want true, nil", ok, err)
	}
	if ok, err := d.Exists(filepath.Join(t.TempDir(), "missing.tidal")); err != nil || ok {
		t.Errorf("Exists(missing) = %v, %v; want false, nil", ok, err)
	}
	if ok, err := d.Exists(t.TempDir()); err != nil || ok {
		t.Errorf("Exists(directory) = %v, %v; want false, nil", ok, err)
	}
	if _, err := d.Exists(""); !errors.Is(err, ErrNoPath) {
		t.Errorf("Exists(empty) error = %v, want ErrNoPath", err)
	}
}

func TestSave_WritesFileAndClearsDirty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")
	want := "d1 $ sound \"bd\"\n"

	d := New()
	if err := d.Save(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("file content = %q, want %q", got, want)
	}
	if d.Path() != path {
		t.Errorf("Path() = %q, want %q", d.Path(), path)
	}
	if d.Dirty(want) {
		t.Error("buffer matching the saved content should not be dirty")
	}
}

func TestSave_EmptyPathUsesBoundPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "set.tidal")

	d := New()
	if err := d.Save(path, "first\n"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save("", "second\n"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second\n" {
		t.Errorf("file content = %q, want %q", got, "second\n")
	}
}

func TestSave_WithoutBoundPath(t *testing.T) {
	d := New()

	err := d.Save("", "x")
	if !errors.Is(err, ErrNoPath) {
		t.Errorf("save without a path should return ErrNoPath, got %v", err)
	}
	if d.Path() != "" {
		t.Errorf("failed save should not bind a path, got %q", d.Path())
	}
}

func TestSave_FailureKeepsDocument(t *testing.T) {
	dir := t.TempDir()

	d := New()
	if err := d.Save(filepath.Join(dir, "set.tidal"), "x\n"); err != nil {
		t.Fatal(err)
	}
	err := d.Save(filepath.Join(dir, "no-such-dir", "set.tidal"), "y\n")
	if err == nil {
		t.Fatal("saving into a missing directory should error")
	}
	if !d.Dirty("y\n") {
		t.Error("failed save should keep the saved content")
	}
}

func TestDirty_UntitledBuffer(t *testing.T) {
	d := New()

	if d.Dirty("") {
		t.Error("empty untitled buffer should not be dirty")
	}
	if !d.Dirty("d1 $ sound \"bd\"") {
		t.Error("typed content in an untitled buffer should be dirty")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	tests := []struct {
		in   string
		want string
	}{
		{in: "~/music/set.tidal", want: filepath.Join(home, "music/set.tidal")},
		{in: "~/", want: home},
		{in: "~", want: home},
		{in: "/tmp/set.tidal", want: "/tmp/set.tidal"},
		{in: "set.tidal", want: "set.tidal"},
		{in: "", want: ""},
	}
	for _, tt := range tests {
		if got := expandHome(tt.in); got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
