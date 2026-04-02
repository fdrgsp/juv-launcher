#!/bin/bash
# macOS .app launcher — opens a file picker for .ipynb/.py files, then runs
# with uvx juv run (Jupyter), uvx marimo run/edit --sandbox (marimo), or uv run (plain .py).

# Extracts marimo_mode from the # /// pyrunner block in a .py file.
# Outputs "run", "edit", or "" if not specified.
marimo_mode() {
  local file="$1" in_block=0
  while IFS= read -r line; do
    if [[ $in_block -eq 0 ]]; then
      [[ "$line" == "# /// pyrunner" ]] && in_block=1
    else
      [[ "$line" == "# ///" ]] && break
      if [[ "$line" =~ ^#[[:space:]]+marimo_mode[[:space:]]*=[[:space:]]*\"([a-z]+)\" ]]; then
        echo "${BASH_REMATCH[1]}"
        return
      fi
    fi
  done < "$file"
}

# Shows a dialog asking the user to choose run or edit mode for a marimo notebook.
# Outputs "run", "edit", or "" on cancel.
ask_marimo_mode() {
  osascript 2>/dev/null << 'EOF'
try
  set msg to "Open marimo notebook in which mode?" & return & return & "  Run - read-only app, outputs and widgets only" & return & "  Edit - full editor, code cells visible and editable"
  set theResult to button returned of (display dialog msg with title "PyRunner" buttons {"Cancel", "Run", "Edit"} default button "Edit")
  if theResult is "Run" then
    return "run"
  else
    return "edit"
  end if
on error
  return ""
end try
EOF
}

# Outputs the run command for the given notebook file path.
select_runner() {
  local notebook="$1"
  case "$notebook" in
    *.ipynb) echo "uvx juv run" ;;
    *.py)
      # Match PEP 723 dependency patterns: "marimo", "marimo>=1", 'marimo', etc.
      # Anchored to quote + "marimo" + (quote or version specifier) to avoid
      # false positives on unrelated strings containing "marimo".
      if grep -qE "[\"']marimo[\"'><=~!]" "$notebook"; then
        local mode
        mode="$(marimo_mode "$notebook")"
        if [[ -z "$mode" ]]; then
          mode="$(ask_marimo_mode)"
        fi
        case "$mode" in
          run)  echo "uvx marimo run --sandbox" ;;
          edit) echo "uvx marimo edit --sandbox" ;;
          *)    echo "" ;;
        esac
      else
        echo "uv run"
      fi
      ;;
  esac
}

_main() {
  local NOTEBOOK
  NOTEBOOK=$(osascript -e 'try
    set theFile to choose file with prompt "Select a notebook (.ipynb or .py):" of type {"public.item"} default location (path to home folder)
    return POSIX path of theFile
  on error
    return ""
  end try' 2>/dev/null)

  if [ -z "$NOTEBOOK" ]; then
    exit 0
  fi

  # Verify it's a supported file type
  case "$NOTEBOOK" in
    *.ipynb|*.py) ;;
    *)
      osascript -e 'display alert "Error" message "Please select a .ipynb or .py file."'
      exit 1
      ;;
  esac

  local NOTEBOOK_DIR NOTEBOOK_NAME RUN_CMD
  NOTEBOOK_DIR="$(dirname "$NOTEBOOK")"
  NOTEBOOK_NAME="$(basename "$NOTEBOOK")"
  RUN_CMD="$(select_runner "$NOTEBOOK")"

  if [ -z "$RUN_CMD" ]; then
    exit 0
  fi

  # Build a temp runner script.  Values are injected via printf '%q' (shell-
  # escaped) so that crafted filenames cannot break out of the script.
  local RUNNER
  RUNNER=$(mktemp /tmp/pyrunner.XXXXXX)
  {
    echo '#!/bin/bash'
    printf 'NB_DIR=%q\n'  "$NOTEBOOK_DIR"
    printf 'NB_NAME=%q\n' "$NOTEBOOK_NAME"
    printf 'NB_CMD=%q\n'  "$RUN_CMD"
    printf 'NB_SELF=%q\n' "$RUNNER"
    cat << 'BODY'
export PATH="$HOME/.local/bin:$PATH"
if ! command -v uv >/dev/null 2>&1; then
  echo "Installing uv..."
  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$PATH"
fi
cd -- "$NB_DIR"
echo "Launching $NB_NAME ..."
eval "$NB_CMD" "$NB_NAME"
rm -f "$NB_SELF"
BODY
  } > "$RUNNER"
  chmod +x "$RUNNER"

  open -a Terminal "$RUNNER"
}

# Allow sourcing this file for testing without running the launcher.
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then _main; fi
