<p align="center">
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=codexpass&amp;subtitle=Use+your+Codex+login+anywhere&amp;logo=openai&amp;logoColor=848484&amp;mode=dark&amp;align=left&amp;font=geist-mono&amp;border=false" /><img alt="codexpass" src="https://shieldcn.dev/header/graph.svg?title=codexpass&amp;subtitle=Use+your+Codex+login+anywhere&amp;logo=openai&amp;logoColor=848484&amp;mode=light&amp;align=left&amp;font=geist-mono&amp;border=false" /></picture>
</p>

<div align="center">

The [Codex CLI](https://github.com/openai/codex) stores a working OpenAI credential on your machine after you log in. **codexpass** reads that credential and lets your other tools use it. You can print shell `export` lines, grab the raw token, or run a local OpenAI-compatible server for editors, SDKs, and scripts. No separate API key to set up.

[Install](#install) · [Usage](#usage) · [Local proxy](#run-a-local-openai-compatible-proxy) · [Configuration](#configuration) · [Releases](https://github.com/hdprajwal/codexpass/releases)

</div>

## How it works

After `codex login`, the Codex CLI saves a working OpenAI credential in
`~/.codex/auth.json`. `codexpass` reads that file. If the token has expired, it
refreshes it with the stored refresh token and writes the new token back safely
with `0600` file permissions. Then it hands you the credential in the shape you
need.

## Install

Download a prebuilt binary for Linux, macOS, or Windows from the
[latest release](https://github.com/hdprajwal/codexpass/releases/latest), or
install with Go:

```bash
go install github.com/hdprajwal/codexpass@latest
```

Or build from source:

```bash
git clone https://github.com/hdprajwal/codexpass
cd codexpass
make build   # produces ./codexpass
```

## Usage

Load the credential into your current shell session:

```bash
eval "$(codexpass export)"
```

Now `OPENAI_API_KEY` (and, in ChatGPT mode, `OPENAI_BASE_URL`) are set for every
tool you run in that session.

Grab the bare token for a script or to paste into code:

```bash
KEY=$(codexpass token)
```

Export only the key, without the base-URL override:

```bash
eval "$(codexpass export --no-base-url)"
```

Check your local Codex login:

```bash
codexpass doctor
codexpass doctor --json
codexpass doctor --live   # also checks the upstream models endpoint
```

## Run a local OpenAI-compatible proxy

`codexpass serve` runs a local OpenAI-compatible server and forwards requests to
the Codex backend. It supports `/v1/chat/completions`, `/v1/responses`,
`/v1/models`, `/healthz`, and optional `/metrics`.

```bash
codexpass serve --port 8080
codexpass serve --port 8080 --token local-secret
codexpass serve --metrics --log-format json --stats-path ~/.cache/codexpass/usage.jsonl
```

### Zed

In Zed `settings.json` (the schema may change between Zed versions, so check
the current docs):

```json
{
  "language_models": {
    "openai_compatible": {
      "Codex": {
        "api_url": "http://localhost:8080/v1",
        "available_models": [
          { "name": "gpt-5.6", "display_name": "GPT-5.6 Sol (Codex)", "max_tokens": 272000 },
          { "name": "gpt-5.6-sol", "display_name": "GPT-5.6 Sol (Codex, explicit)", "max_tokens": 272000 },
          { "name": "gpt-5.6-terra", "display_name": "GPT-5.6 Terra (Codex)", "max_tokens": 272000 },
          { "name": "gpt-5.6-luna", "display_name": "GPT-5.6 Luna (Codex)", "max_tokens": 272000 }
        ]
      }
    }
  }
}
```

When Zed asks for an API key, enter the `--token` value, or any placeholder if
you did not set one. The real Codex credential stays inside the proxy.

A few caveats. Requests count against your ChatGPT subscription quota. The
Codex backend does not serve embeddings, images, or audio; those endpoints
return `501` unless you explicitly configure a fallback backend. The proxy is
meant for personal, local use. Keep it bound to loopback (the default) and do
not expose it to others.

### Models

List models available to your current Codex credential:

```bash
codexpass models list
```

GPT-5.6 has three tiers: `gpt-5.6-sol` for frontier capability,
`gpt-5.6-terra` for a balance of intelligence and cost, and `gpt-5.6-luna`
for efficient high-volume work. When Sol is available, codexpass also exposes
OpenAI's official `gpt-5.6` alias and resolves it to `gpt-5.6-sol` if the
upstream model catalog does not already provide the alias.

You can also define model aliases. This lets a client ask for a local name while
codexpass sends the real Codex model name upstream.

```json
{
  "models": {
    "cache_ttl_seconds": 300,
    "aliases": {
      "gpt-codex": "gpt-5.6-sol"
    }
  }
}
```

With this config, clients can request `gpt-codex`. codexpass forwards
`gpt-5.6-sol` upstream.

### Proxy compatibility

The proxy accepts common OpenAI chat/completions fields used by SDKs and
editors: `model`, `messages`, `stream`, `stream_options.include_usage`,
`temperature`, `top_p`, `max_tokens`, `max_completion_tokens`,
`reasoning_effort`, `safety_identifier`, `tools`, `tool_choice`, and
`response_format`. GPT-5.6 reasoning efforts through `max` and image detail
values including `original` are preserved. User text, image parts, function
tools, tool results, and structured output are translated to the Responses API.

These fields are accepted for compatibility but ignored by the Codex-backed
route today: `metadata`, `user`, `n`, `presence_penalty`,
`frequency_penalty`, `stop`, `logit_bias`, and `seed`. Invalid request shape,
unsupported message roles, non-function tools, and malformed `tool_choice`
return OpenAI-shaped `invalid_request_error` responses.

`/v1/responses` is proxied too. JSON request bodies are validated without
rewriting unknown fields, aliases are resolved, and `store` defaults to `false`
when the client does not set it. GPT-5.6 features such as Pro mode, persisted
reasoning, explicit prompt caching, and Programmatic Tool Calling should use
this endpoint. The `OpenAI-Beta` request header is forwarded for opt-in features
such as the multi-agent beta; client-supplied authorization and account headers
are never forwarded upstream.

### Client tokens and policy

For one local secret, use `--token`:

```bash
codexpass serve --token local-secret
```

For multiple local clients, generate a config snippet:

```bash
codexpass token create zed
```

Client policy can restrict endpoints, models, request size, fallback use, and
simple per-minute rate limits:

```json
{
  "clients": {
    "zed": {
      "token": "generated-local-token",
      "allowed_endpoints": ["models", "chat.completions", "responses"],
      "allowed_models": ["gpt-codex", "gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"],
      "max_body_bytes": 1048576,
      "rate_limit_per_minute": 60,
      "allow_fallback": false
    }
  }
}
```

`/healthz` stays public. When any token or client policy is configured, all
other endpoints require a matching bearer token.

### Observability

Verbose logging records request metadata only:

```bash
codexpass serve --verbose
codexpass serve --log-format json
```

Logs include route, status, latency, and client metadata. They do not include
prompts, completions, tool arguments, Authorization headers, API keys, OAuth
access tokens, refresh tokens, or id tokens.

Enable Prometheus-style metrics:

```bash
codexpass serve --metrics
curl http://localhost:8080/metrics
```

Write redacted usage events and summarize them later:

```bash
codexpass serve --stats-path ~/.cache/codexpass/usage.jsonl
codexpass stats --path ~/.cache/codexpass/usage.jsonl
```

### Fallback backend

Unsupported endpoints can route to a separate OpenAI-compatible backend. This
is off by default.

```json
{
  "fallback": {
    "enabled": true,
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

When enabled, `/v1/embeddings`, `/v1/images/generations`, `/v1/audio/speech`,
and `/v1/audio/transcriptions` go to the fallback backend. Those requests may
use a different quota or billing account.

### Background service

Install codexpass as a user-level background service:

```bash
codexpass service install --config ~/.config/codexpass/config.json
codexpass service status
codexpass service uninstall
```

Use `--dry-run` to print the generated systemd user unit or macOS launchd plist:

```bash
codexpass service install --dry-run
```

Service installation refuses non-loopback hosts unless you pass `--allow-network`.

## What kind of key you get

Codex stores credentials in one of two modes:

- **`chatgpt` mode**: you logged in with a ChatGPT subscription. The borrowed
  value is a ChatGPT OAuth access token, not a normal `sk-...` key. It only
  works against the Codex backend, so `codexpass export` also sets
  `OPENAI_BASE_URL=https://chatgpt.com/backend-api/codex`, and you have to use
  Codex model names (`gpt-5.x`). Some tools also send a `ChatGPT-Account-ID`
  header. That header cannot be passed through an environment variable, so
  tools that do not send it may fail. `codexpass` prints your account id to
  stderr as a reminder.
- **`apikey` mode**: you logged in with an API key. The borrowed value is a
  real OpenAI API key that works against `api.openai.com`. `codexpass` exports
  just `OPENAI_API_KEY` and leaves the base URL alone.

## Commands

| Command | Description |
| --- | --- |
| `codexpass export [--no-base-url]` | Print eval-able `export` lines to stdout; notes go to stderr. |
| `codexpass token` | Print the bare borrowed token to stdout. |
| `codexpass token create NAME` | Generate a local proxy client-token config snippet. |
| `codexpass doctor [--json] [--live]` | Inspect local Codex auth state without printing secrets. |
| `codexpass models list` | List available models, including configured aliases. |
| `codexpass stats --path PATH` | Summarize redacted usage JSONL. |
| `codexpass serve [--host H] [--port N] [--token S] [--config PATH]` | Run the local OpenAI-compatible server. |
| `codexpass service install\|uninstall\|status` | Manage a user-level background proxy service. |
| `codexpass --version` | Print the version. |
| `codexpass --help` | Show help. |

## Configuration

- `CODEX_HOME`: override the Codex home directory. The default is `~/.codex`.
- `CODEXPASS_CONFIG`: override the codexpass config path.

The default config path is `$XDG_CONFIG_HOME/codexpass/config.json`, or
`~/.config/codexpass/config.json` when `XDG_CONFIG_HOME` is unset. If the file
does not exist, codexpass uses its defaults.

Example:

```json
{
  "server": {
    "host": "127.0.0.1",
    "port": 8080,
    "log_format": "json",
    "metrics": true,
    "stats_path": "/home/me/.cache/codexpass/usage.jsonl",
    "retry_attempts": 3
  },
  "models": {
    "cache_ttl_seconds": 300,
    "aliases": {
      "gpt-codex": "gpt-5.6-sol"
    }
  },
  "clients": {
    "zed": {
      "token": "generated-local-token",
      "allowed_endpoints": ["models", "chat.completions", "responses"],
      "allowed_models": ["gpt-codex", "gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"],
      "max_body_bytes": 1048576,
      "rate_limit_per_minute": 60,
      "allow_fallback": false
    }
  },
  "fallback": {
    "enabled": false,
    "base_url": "https://api.openai.com/v1",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

`codexpass serve` flags override config-file server values.

## Development

```bash
make test    # go test ./...
make vet     # go vet ./...
make build   # build ./codexpass
```

## Credits

The credential-borrowing and token-refresh logic is ported from Simon
Willison's [llm-openai-via-codex](https://github.com/simonw/llm-openai-via-codex).

<div align="center">

[Apache-2.0](LICENSE) · Built by [HD Prajwal](https://github.com/hdprajwal) · [Contribute](https://github.com/hdprajwal/codexpass)

</div>
