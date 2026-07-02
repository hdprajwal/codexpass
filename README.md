<p align="center">
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://shieldcn.dev/header/graph.svg?title=codexpass&amp;subtitle=Use+your+Codex+login+anywhere&amp;logo=openai&amp;logoColor=848484&amp;mode=dark&amp;align=left&amp;font=geist-mono&amp;border=false" /><img alt="codexpass" src="https://shieldcn.dev/header/graph.svg?title=codexpass&amp;subtitle=Use+your+Codex+login+anywhere&amp;logo=openai&amp;logoColor=848484&amp;mode=light&amp;align=left&amp;font=geist-mono&amp;border=false" /></picture>
</p>

<div align="center">

The [Codex CLI](https://github.com/openai/codex) stores a working OpenAI credential on your machine after you log in. **codexpass** reads that credential and lets your other tools use it. You can print it as shell `export` lines, grab it as a raw token, or run a small local server that lets editors like Zed use your ChatGPT subscription. No separate API key to set up.

[Install](#install) · [Usage](#usage) · [Local proxy](#run-a-local-openai-compatible-proxy) · [Releases](https://github.com/hdprajwal/codexpass/releases)

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

## Run a local OpenAI-compatible proxy

`codexpass serve` runs a small local server that speaks the OpenAI
chat/completions API and forwards requests to the Codex backend. Tools that
support `/v1/chat/completions`, like the Zed editor, can then use your Codex
subscription.

```bash
codexpass serve --port 8080            # add --token <secret> to require a key
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
          { "name": "gpt-5.4", "display_name": "GPT-5.4 (Codex)", "max_tokens": 128000 }
        ]
      }
    }
  }
}
```

When Zed asks for an API key, enter the `--token` value, or any placeholder if
you did not set one. The real Codex credential stays inside the proxy.

A few caveats. Requests count against your ChatGPT subscription quota. The
proxy only serves chat, so no embeddings, images, or audio. It is meant for
personal, single-user use. Keep it bound to loopback (the default) and do not
expose it to others.

### Proxy compatibility

The proxy accepts the common OpenAI chat/completions fields used by SDKs and
editors: `model`, `messages`, `stream`, `stream_options.include_usage`,
`temperature`, `top_p`, `max_tokens`, `max_completion_tokens`,
`reasoning_effort`, `tools`, `tool_choice`, and `response_format`. User text,
image parts, function tools, tool results, and structured output are translated
to the Responses API.

These OpenAI fields are accepted for client compatibility but ignored by the
Codex-backed route today: `metadata`, `user`, `n`, `presence_penalty`,
`frequency_penalty`, `stop`, `logit_bias`, and `seed`. Invalid request shape,
unsupported message roles, non-function tools, and malformed `tool_choice`
return OpenAI-shaped `invalid_request_error` responses.

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
| `codexpass token` | Print the bare token to stdout. |
| `codexpass serve [--host H] [--port N] [--token S]` | Run a local OpenAI-compatible proxy to the Codex backend. |
| `codexpass --version` | Print the version. |
| `codexpass --help` | Show help. |

## Configuration

- `CODEX_HOME`: override the Codex home directory. The default is `~/.codex`.

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
