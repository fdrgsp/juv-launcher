package main

import (
	"os"
	"path/filepath"
	"testing"
)

var selectRunnerTests = []struct {
	name     string
	filename string
	content  string
	expected string
}{
	{"ipynb uses juv", "notebook.ipynb", "", "uvx juv run"},
	{"py with marimo dep edit mode", "nb.py", "# /// pyrunner\n# marimo_mode = \"edit\"\n# ///\n# dependencies = [\n#   \"marimo\",\n# ]", "uvx marimo edit --sandbox"},
	{"py with marimo dep run mode", "nb.py", "# /// pyrunner\n# marimo_mode = \"run\"\n# ///\n# dependencies = [\n#   \"marimo\",\n# ]", "uvx marimo run --sandbox"},
	{"py without marimo uses uv run", "script.py", "# dependencies = [\n#   \"numpy\",\n# ]", "uv run"},
	{"py with empty content uses uv run", "script.py", "", "uv run"},
	{"py with marimo version spec edit mode", "nb.py", "# /// pyrunner\n# marimo_mode = \"edit\"\n# ///\n# dependencies = [\n#   \"marimo>=0.1\",\n# ]", "uvx marimo edit --sandbox"},
	{"py with single-quoted marimo edit mode", "nb.py", "# /// pyrunner\n# marimo_mode = \"edit\"\n# ///\n# dependencies = [\n#   'marimo',\n# ]", "uvx marimo edit --sandbox"},
	{"py with unrelated marimo mention uses uv run", "script.py", "# this is not marimo_extra related", "uv run"},
}

func TestSelectRunner(t *testing.T) {
	for _, tc := range selectRunnerTests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.filename)
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("setup: %v", err)
			}
			got := selectRunner(path)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

var marimoModeTests = []struct {
	name     string
	content  string
	expected string
}{
	{"no pyrunner block", "# dependencies = [\n#   \"marimo\",\n# ]", ""},
	{"run mode", "# /// pyrunner\n# marimo_mode = \"run\"\n# ///\n", "run"},
	{"edit mode", "# /// pyrunner\n# marimo_mode = \"edit\"\n# ///\n", "edit"},
	{"block without marimo_mode", "# /// pyrunner\n# other_key = \"value\"\n# ///\n", ""},
	{"marimo_mode after other keys", "# /// pyrunner\n# other = \"x\"\n# marimo_mode = \"run\"\n# ///\n", "run"},
}

func TestMarimoMode(t *testing.T) {
	for _, tc := range marimoModeTests {
		t.Run(tc.name, func(t *testing.T) {
			got := marimoMode(tc.content)
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
