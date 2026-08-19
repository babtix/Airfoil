All public functions accept a concrete file path string; callers construct paths (typically under data/).
WriteJSON always produces indented JSON with two-space indentation and a final newline.
Atomicity is mandatory: every write goes through WriteFile's temp-file + rename sequence.
ReadJSONOr only swallows fs.ErrNotExist; any JSON syntax error is returned to the caller to avoid silent corruption.
Errors are wrapped with fmt.Errorf using the prefix "store:" and include the path for debugging.
Directory creation uses 0o755 permissions; temp files inherit the directory's default umask.
No global configuration or initialization — pure functions.
