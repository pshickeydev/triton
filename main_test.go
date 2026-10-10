package main

import (
	"testing"

	"github.com/pshickeydev/triton/internal/boot"
)

func TestBundledCursorScriptCarriesEditorTarget(t *testing.T) {
	_, cleanup, err := boot.Cursor("", cursorBootScript, "127.0.0.1", 6013)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "no args", args: []string{"triton"}, want: ""},
		{name: "file", args: []string{"triton", "set.tidal"}, want: "set.tidal"},
		{name: "flag", args: []string{"triton", "-x"}, wantErr: true},
		{name: "extra", args: []string{"triton", "a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parseArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
