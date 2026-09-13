// Package doc binds the editor buffer to a .tidal file: loading a file
// into the buffer, saving the buffer back to it, and telling unsaved
// changes from saved content.
package doc

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	filePerm = 0o644

	// Atomic saves stage content in a hidden sibling of the target.
	tmpSuffix = ".tmp"

	// The editor buffer sanitizes input the same way the Bubbles
	// textarea does (runeutil defaults): carriage returns collapse to
	// newlines, tabs to spaces, and other control characters and
	// invalid UTF-8 bytes are dropped. Loading normalizes identically
	// so the saved copy always matches the buffer, CRLF files included.
	dosLineEnd  = "\r\n"
	macLineEnd  = "\r"
	unixLineEnd = "\n"
	tabStop     = "    "
)

var ErrNoPath = errors.New("no file path")

type Doc struct {
	path  string
	saved string
}

func New() *Doc {
	return &Doc{}
}

// Load reads the file at path and binds it as the current document.
// On error the document is left unchanged.
func (d *Doc) Load(path string) (string, error) {
	p, err := resolve(path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	d.path = p
	d.saved = normalize(string(content))
	return d.saved, nil
}

// Save writes content to path atomically. An empty path saves to the
// bound file. The document is only rebound on success.
func (d *Doc) Save(path, content string) error {
	p := d.path
	if path != "" {
		resolved, err := resolve(path)
		if err != nil {
			return err
		}
		p = resolved
	}
	if p == "" {
		return ErrNoPath
	}
	if err := writeFile(p, content); err != nil {
		return err
	}
	d.path = p
	d.saved = content
	return nil
}

// writeFile saves atomically: content goes to a hidden temp file next to
// path, which is flushed and then renamed over it, so a failed or
// interrupted save never truncates the previous file. Symlinks are
// followed, an existing file's permissions are preserved, and new files
// get filePerm through the umask.
func writeFile(path, content string) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if err := checkWritable(path); err != nil {
		return err
	}
	f, err := createTemp(path)
	if err != nil {
		return err
	}
	tmp := f.Name()
	if info, statErr := os.Stat(path); statErr == nil {
		err = f.Chmod(info.Mode().Perm())
	}
	if err == nil {
		_, err = f.WriteString(content)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// checkWritable opens path the way a direct write would, so read-only
// files keep failing to save. A missing file is fine: it gets created.
func checkWritable(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}

// createTemp creates the hidden temp file that writeFile renames into
// place.
func createTemp(path string) (*os.File, error) {
	dir, base := filepath.Split(path)
	name := filepath.Join(dir, fmt.Sprintf(".%s.%d%s", base, rand.Int64(), tmpSuffix))
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, filePerm)
}

// Path returns the bound file path, empty while unsaved.
func (d *Doc) Path() string {
	return d.path
}

// Content returns the loaded or last saved content.
func (d *Doc) Content() string {
	return d.saved
}

// Exists reports whether path names an existing file (not a directory),
// resolving ~ and relative paths exactly like Load and Save.
func (d *Doc) Exists(path string) (bool, error) {
	p, err := resolve(path)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// Dirty reports whether the buffer content differs from the last saved
// or loaded content.
func (d *Doc) Dirty(content string) bool {
	return content != d.saved
}

func normalize(content string) string {
	content = strings.ReplaceAll(content, dosLineEnd, unixLineEnd)
	content = strings.ReplaceAll(content, macLineEnd, unixLineEnd)
	content = strings.ReplaceAll(content, "\t", tabStop)
	return strings.Map(dropControl, content)
}

// dropControl mirrors the editor's rune sanitizer: control characters
// other than newline never reach the buffer, and invalid UTF-8 bytes
// decode as RuneError and are dropped the same way.
func dropControl(r rune) rune {
	if r == '\n' {
		return r
	}
	if r == utf8.RuneError || unicode.IsControl(r) {
		return -1
	}
	return r
}

func resolve(path string) (string, error) {
	p := expandHome(strings.TrimSpace(path))
	if p == "" {
		return "", ErrNoPath
	}
	return filepath.Abs(p)
}

func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
