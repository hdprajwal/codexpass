# codex2key

Borrow the OpenAI credential your [Codex CLI](https://github.com/openai/codex)
already has, and use it with any tool that reads `OPENAI_API_KEY`.

After `codex login`, the Codex CLI stores a working OpenAI credential in
`~/.codex/auth.json`. `codex2key` reads that credential — refreshing it if it has
expired — and hands it to you as shell `export` lines or a raw token. No separate
API key to provision.

This is a small Go port of the `borrow_codex_key()` logic from
[simonw/llm-openai-via-codex](https://github.com/simonw/llm-openai-via-codex),
turned into a standalone CLI.

## Install

```bash
go install github.com/hdprajwal/codex2key@latest
```

Or build from source:

```bash
git clone https://github.com/hdprajwal/codex2key
cd codex2key
make build   # produces ./codex2key
```

## Usage

Inject the credential into your current shell session:

```bash
eval "$(codex2key export)"
```

Now `OPENAI_API_KEY` (and, in ChatGPT mode, `OPENAI_BASE_URL`) are set for every
tool you run in that session.

Grab the bare token for a script or to paste into code:

```bash
KEY=$(codex2key token)
```

Export only the key, without the base-URL override:

```bash
eval "$(codex2key export --no-base-url)"
```

## Important: what kind of key you get

Codex stores credentials in one of two modes:

- **`chatgpt` mode** (you logged in with a ChatGPT subscription): the borrowed
  value is a **ChatGPT OAuth access token**, not a normal `sk-...` key. It only
  works against the Codex backend, so `codex2key export` also sets
  `OPENAI_BASE_URL=https://chatgpt.com/backend-api/codex` and you must use Codex
  model names (`gpt-5.x`). Some tools additionally send a `ChatGPT-Account-ID`
  header, which cannot be passed through an environment variable — tools that
  don't send it may fail. `codex2key` prints your account id to stderr as a
  reminder.
- **`apikey` mode** (you logged in with an API key): the borrowed value is a real
  OpenAI API key that works against `api.openai.com`. `codex2key` exports just
  `OPENAI_API_KEY` and leaves the base URL alone.

Expired ChatGPT tokens are refreshed automatically using the refresh token in
`auth.json`, and the refreshed tokens are written back atomically with `0600`
permissions.

## Configuration

- `CODEX_HOME` — override the Codex home directory (default `~/.codex`).

## Commands

| Command | Description |
| --- | --- |
| `codex2key export [--no-base-url]` | Print eval-able `export` lines to stdout; notes go to stderr. |
| `codex2key token` | Print the bare token to stdout. |
| `codex2key --version` | Print the version. |
| `codex2key --help` | Show help. |

## Development

```bash
make test    # go test ./...
make vet     # go vet ./...
make build   # build ./codex2key
```

## Credits

The credential-borrowing and token-refresh logic is ported from Simon Willison's
[llm-openai-via-codex](https://github.com/simonw/llm-openai-via-codex).

## License

Apache-2.0. See [LICENSE](LICENSE).
