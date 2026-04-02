// Windows .exe launcher — opens a file picker for .ipynb/.py files, then runs
// with uvx juv run (Jupyter), uvx marimo run/edit --sandbox (marimo), or uv run (plain .py).

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// marimoMode reads the # /// pyrunner block from file content and returns
// the marimo_mode value ("run", "edit", or "" if not set).
func marimoMode(content string) string {
	inBlock := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if !inBlock {
			if line == "# /// pyrunner" {
				inBlock = true
			}
		} else {
			if line == "# ///" {
				break
			}
			if strings.HasPrefix(line, "# marimo_mode") {
				eqIdx := strings.Index(line, "=")
				if eqIdx >= 0 {
					val := strings.TrimSpace(line[eqIdx+1:])
					val = strings.Trim(val, `"'`)
					return val
				}
			}
		}
	}
	return ""
}

// askMarimoMode shows a dialog asking the user to choose open mode for a marimo notebook.
// Returns "run", "edit", or "" on cancel.
func askMarimoMode() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", `
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$form = New-Object System.Windows.Forms.Form
$form.Text = "PyRunner"
$form.Size = New-Object System.Drawing.Size(420, 200)
$form.StartPosition = "CenterScreen"
$form.FormBorderStyle = "FixedDialog"
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$label = New-Object System.Windows.Forms.Label
$label.Text = "Open marimo notebook in which mode?"
$label.Font = New-Object System.Drawing.Font("Segoe UI", 10)
$label.Location = New-Object System.Drawing.Point(20, 20)
$label.Size = New-Object System.Drawing.Size(380, 24)
$form.Controls.Add($label)
$desc = New-Object System.Windows.Forms.Label
$desc.Text = "Run: read-only app, outputs and widgets only`nEdit: full editor, code cells visible and editable"
$desc.Location = New-Object System.Drawing.Point(20, 50)
$desc.Size = New-Object System.Drawing.Size(380, 40)
$form.Controls.Add($desc)
$btnRun = New-Object System.Windows.Forms.Button
$btnRun.Text = "Run"
$btnRun.Location = New-Object System.Drawing.Point(20, 110)
$btnRun.Size = New-Object System.Drawing.Size(100, 30)
$btnRun.Add_Click({ $form.Tag = "run"; $form.Close() })
$form.Controls.Add($btnRun)
$btnEdit = New-Object System.Windows.Forms.Button
$btnEdit.Text = "Edit"
$btnEdit.Location = New-Object System.Drawing.Point(140, 110)
$btnEdit.Size = New-Object System.Drawing.Size(100, 30)
$btnEdit.Add_Click({ $form.Tag = "edit"; $form.Close() })
$form.Controls.Add($btnEdit)
$btnCancel = New-Object System.Windows.Forms.Button
$btnCancel.Text = "Cancel"
$btnCancel.Location = New-Object System.Drawing.Point(290, 110)
$btnCancel.Size = New-Object System.Drawing.Size(100, 30)
$btnCancel.Add_Click({ $form.Tag = ""; $form.Close() })
$form.Controls.Add($btnCancel)
$form.ShowDialog() | Out-Null
$form.Tag`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// selectRunner returns the run command for the given notebook file path.
func selectRunner(notebookPath string) string {
	if strings.HasSuffix(notebookPath, ".ipynb") {
		return "uvx juv run"
	}
	content, err := os.ReadFile(notebookPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot read %s: %v\n", notebookPath, err)
		return "uv run"
	}
	if isMarimo(string(content)) {
		mode := marimoMode(string(content))
		if mode == "" {
			mode = askMarimoMode()
		}
		switch mode {
		case "run":
			return "uvx marimo run --sandbox"
		case "edit":
			return "uvx marimo edit --sandbox"
		default:
			return "" // user cancelled
		}
	}
	return "uv run"
}

// isMarimo reports whether file content declares a marimo dependency.
// It matches PEP 723 / TOML patterns like "marimo", 'marimo', "marimo>=1.0".
func isMarimo(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Match common dependency declaration patterns:
		//   "marimo"  "marimo>=1.0"  'marimo'  'marimo>=1.0'
		if strings.Contains(trimmed, `"marimo"`) ||
			strings.Contains(trimmed, `"marimo>`) ||
			strings.Contains(trimmed, `"marimo<`) ||
			strings.Contains(trimmed, `"marimo=`) ||
			strings.Contains(trimmed, `"marimo~`) ||
			strings.Contains(trimmed, `"marimo!`) ||
			strings.Contains(trimmed, `'marimo'`) ||
			strings.Contains(trimmed, `'marimo>`) ||
			strings.Contains(trimmed, `'marimo<`) ||
			strings.Contains(trimmed, `'marimo=`) ||
			strings.Contains(trimmed, `'marimo~`) ||
			strings.Contains(trimmed, `'marimo!`) {
			return true
		}
	}
	return false
}

func main() {
	// Show file picker for .ipynb and .py files
	out, err := exec.Command("powershell", "-NoProfile", "-Command", `
		Add-Type -AssemblyName System.Windows.Forms
		$dlg = New-Object System.Windows.Forms.OpenFileDialog
		$dlg.Title = "Select a notebook (.ipynb or .py)"
		$dlg.Filter = "Notebooks (*.ipynb;*.py)|*.ipynb;*.py|Jupyter Notebooks (*.ipynb)|*.ipynb|Python Scripts (*.py)|*.py"
		$dlg.InitialDirectory = [Environment]::GetFolderPath('UserProfile')
		if ($dlg.ShowDialog() -eq 'OK') { $dlg.FileName } else { "" }
	`).Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error showing file dialog: %v\n", err)
		os.Exit(1)
	}

	selected := strings.TrimSpace(string(out))
	if selected == "" {
		os.Exit(0)
	}

	runCmd := selectRunner(selected)
	if runCmd == "" {
		os.Exit(0)
	}

	notebookDir := filepath.Dir(selected)
	notebook := filepath.Base(selected)

	// Sanitize values for safe batch-file interpolation: double any '%' so
	// cmd.exe doesn't treat them as variable references, and quote paths.
	safeDirArg := strings.ReplaceAll(notebookDir, "%", "%%")
	safeNameArg := strings.ReplaceAll(notebook, "%", "%%")

	// Bootstrap uv if needed, then run
	script := fmt.Sprintf(`@echo off
powershell -ExecutionPolicy Bypass -Command "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force" >nul 2>&1
where uv >nul 2>&1 || (
    echo Installing uv...
    powershell -ExecutionPolicy Bypass -c "irm https://astral.sh/uv/install.ps1 | iex"
)
cd /d "%s"
echo Launching %s ...
%s "%s"
pause
`, safeDirArg, safeNameArg, runCmd, safeNameArg)

	// Use a unique temp file to avoid races when launched multiple times.
	batFile, err := os.CreateTemp("", "pyrunner-*.bat")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp file: %v\n", err)
		os.Exit(1)
	}
	batPath := batFile.Name()
	batFile.WriteString(script)
	batFile.Close()

	cmd := exec.Command("cmd", "/c", batPath)
	cmd.Dir = notebookDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Run()

	os.Remove(batPath)
}
