// Package boot resolves the boot scripts triton hands to GHCi and
// sclang: configured overrides win, otherwise the bundled scripts are
// rewritten with the configured addresses and written to temp files.
package boot

import (
	"fmt"
	"os"
	"strings"

	"github.com/pshickeydev/triton/internal/config"
)

// editorTargetAddr is the unique address fragment of the bundled
// cursor boot script's editor target, used to carry the configured
// listener host and port into the GHCi session.
const editorTargetAddr = `oAddress = "127.0.0.1", oPort = 6013`

// Cursor resolves the boot script for the cursor-enabled GHCi session:
// the configured override when set (its own responsibility to match the
// listener address), otherwise the bundled script rewritten with the
// configured host and port and written to a temp file. The returned
// cleanup removes the temp file.
func Cursor(override, embedded, host string, port int) (string, func(), error) {
	if override != "" {
		return override, func() {}, nil
	}
	if strings.Count(embedded, editorTargetAddr) != 1 {
		return "", nil, fmt.Errorf("bundled boot script lacks the editor target address")
	}
	script := strings.Replace(
		embedded,
		editorTargetAddr,
		fmt.Sprintf("oAddress = %q, oPort = %d", host, port),
		1,
	)
	return writeTemp("BootTidal-triton-*.hs", script)
}

// SuperDirt resolves the SuperDirt boot script: the configured path when
// that file exists, otherwise the bundled script written to a temp file
// when the config still carries the default name (so installed binaries
// work outside the project directory). Any other missing path is
// returned unchanged for the dependency check to report. The returned
// cleanup removes the temp file.
func SuperDirt(configured, embedded string) (string, func(), error) {
	if _, err := os.Stat(configured); err == nil || configured != config.DefaultSuperDirtBootScript {
		return configured, func() {}, nil
	}
	return writeTemp("start-superdirt-*.scd", embedded)
}

func writeTemp(pattern, content string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", nil, err
	}
	return f.Name(), func() { os.Remove(f.Name()) }, nil
}
