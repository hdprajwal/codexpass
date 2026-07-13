package codex

// CodexOriginator and the matching user agent are part of the Codex backend
// transport contract. Some models are routed only for recognized Codex clients
// even when the models catalog marks them supported_in_api. These values are
// owned by codexpass; serving requests never executes or proxies through the
// Codex CLI.
const (
	CodexOriginator    = "codex_cli_rs"
	CodexClientVersion = "0.144.3"
	CodexUserAgent     = CodexOriginator + "/" + CodexClientVersion
)
