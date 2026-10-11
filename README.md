# gyazo-cli

`gy` is a command line client for [Gyazo](https://gyazo.com), written in Go.
It is a sibling of [scrapbox-cli](https://github.com/hiroki1117/scrapbox-cli) (`sb`) and shares its conventions.

## Features

- Upload images from files or standard input, and print the link as a permalink, image URL, Markdown or Scrapbox notation
- List, show, search (Gyazo Pro) and delete your images
- Print the OCR text of an image
- Download image files safely (no partial files, no accidental overwrites)
- Print results as JSON with `-o json` for `jq` and AI agents

## Install

| Method | Steps |
|---|---|
| Binary | Download the archive for your OS from [Releases](https://github.com/hiroki1117/gyazo-cli/releases) and put `gy` on your PATH |
| go install | `go install github.com/hiroki1117/gyazo-cli/cmd/gy@latest` (Go 1.27+) |
| From source | `git clone`, then `make build` (creates `bin/gy`) |

Binaries are available for Linux, macOS and Windows on amd64 and arm64. Run `gy version` to check the version.

## Quick start

```bash
gy auth login                     # 1. Save your access token
gy upload screenshot.png          # 2. Upload an image and print its URL
gy image list                     # 3. List your images
```

Run `gy --help` for the list of commands and `gy <command> --help` for details.

## Configuration

Settings come from flags, environment variables and the config file. The precedence is **flags > environment variables > config file**.

### Authentication

Every API request needs an OAuth access token.

1. Open [https://gyazo.com/oauth/applications](https://gyazo.com/oauth/applications) and register an application (any name; the callback URL is not used)
2. Open the application and click "Your Access Token"
3. Save it with `gy auth login`, or set `GYAZO_ACCESS_TOKEN`

```bash
gy auth login                          # Enter the token at the hidden prompt
pbpaste | gy auth login --with-token   # Read from stdin
gy auth status                         # Show the current auth state (token masked)
gy auth logout                         # Remove from the config file
```

- `login` verifies the token against the API before saving. Use `--no-verify` to skip this.
- Tokens are never accepted as command line arguments, to keep them out of shell history.
- If `GYAZO_ACCESS_TOKEN` is set, the token in the config file is ignored.

### Config file

`~/.config/gy/config.json` (or `$XDG_CONFIG_HOME/gy/config.json`) is created with permission 0600. Change the path with `GY_CONFIG`.

```bash
gy config set upload.access-policy only_me   # Make uploads private by default
gy config set upload.app my-script           # Application name recorded with uploads
gy config get upload.access-policy
gy config unset upload.app
gy config path
```

### Environment variables

| Variable | Description | Default |
|---|---|---|
| `GYAZO_ACCESS_TOKEN` | OAuth access token | - |
| `GYAZO_API_URL` | API base URL | `https://api.gyazo.com` |
| `GYAZO_UPLOAD_URL` | Upload API base URL | `https://upload.gyazo.com` |
| `GYAZO_TIMEOUT` | Request timeout (e.g. `10s`) | `30s` |
| `GYAZO_MAX_RETRIES` | Max retries on 429/5xx/network errors | `3` |
| `GY_CONFIG` | Config file path | `~/.config/gy/config.json` |

## Usage

`<id|url>` is an image ID, or a Gyazo URL such as `https://gyazo.com/<id>`, `https://i.gyazo.com/<id>.png` or `https://<team>.gyazo.com/<id>`.

```bash
# Upload (files or "-" for stdin)
gy upload screenshot.png                    # Prints the permalink
gy upload *.png --as scrapbox | pbcopy      # Link style: permalink|url|markdown|scrapbox
pngpaste - | gy upload -                    # From the clipboard (macOS, with pngpaste)
gy upload shot.png --title "Design review" --desc "#ui" --access-policy only_me

# Images
gy image list                               # Newest first (--page, --per-page, --all)
gy image list --per-page 5 --as markdown
gy image get <id|url>                       # URLs, capture metadata
gy image ocr <id|url>                       # Recognized text
gy image download <id|url>... --dir backup  # Saves <id>.<ext>; --force to overwrite
gy image download <id|url> --stdout > a.png
gy image delete <id|url>...                 # Shows the images; add --yes to delete

# Search (Gyazo Pro)
gy search invoice 2026 --per 50

# Other
gy whoami
gy version
gy help environment                         # Environment variables and precedence
gy completion zsh > "${fpath[1]}/_gy"       # Shell completion (bash / zsh / fish / powershell)
```

### Output format

`-o json` prints JSON that keeps the API field names (`image_id`, `permalink_url`, ...). It is meant to be used with `jq`.

```bash
gy image list --all -o json | jq -r '.images[] | select(.metadata.app == "Chrome") | .permalink_url'
gy upload a.png -o json | jq -r '.[0].url'
```

`image download --stdout` always prints the raw image data regardless of `-o`.

## Limitations

These come from the [Gyazo API](https://gyazo.com/api/docs), not from `gy`.

- `gy search` requires a Gyazo Pro plan. A query is up to 200 characters, and only the first 10,000 results can be retrieved.
- The API allows 12,500 requests per day. `gy image list --all` sends one request per 100 images.
- The public API cannot edit an image after upload (title, description, tags) or manage collections. `--collection` only adds a new upload to a collection.

Not implemented yet: the OAuth login flow (use an access token), oEmbed, and opening an image in the browser.

## Design

```
cmd/gy/            Entry point (signals, exit code, error hints)
internal/
  cli/             cobra commands. App passes the config, Client and Printer to each command
  config/          Loads the config file and environment variables; saves the config file
  output/          Text / JSON output, table and time formatting (same as scrapbox-cli)
  gyazo/           API client (auth, retries, error conversion, APIs)
```

Dependencies flow `cmd → cli → (config, output, gyazo)`. The `gyazo` package depends on no other package in this module.

### Extending

- **Add an API**: Add a method to the matching service in `gyazo` (such as `ImagesService`). For a new resource, define `type XxxService service` and register it in `NewClient`. Use `getJSON`, or `newAPIRequest` + `doJSON` for other methods.
- **Add a command**: Define `newXxxCmd(app *App) *cobra.Command` in `internal/cli` and add it to its parent with `AddCommand`. Print with `app.Printer().Print(v, textFunc)` to support `-o json` automatically. Write `Long` (including the output format) and `Example` (`help_test.go` checks this for image commands).
- **Add a link style**: Add a function to `linkStyles` in `internal/cli/link.go`. It is available to `--as` of every command.
- **Add an auth method**: Implement `gyazo.Authenticator` and return it from `config.Credentials.Authenticator`.
- **Add a setting**: Add a field to `config.File` and register it in `configKeys` in `internal/cli/configcmd.go` (with an optional `validate`).

### Uploads

The multipart body is streamed from the file with an exact `Content-Length`, so large files are not read into memory. Only standard input is buffered, because its size is unknown.
The content type is detected from the data, not the file extension.

### Retries

429 / 502 / 503 / 504 and network errors are retried up to 3 times with exponential backoff (with jitter).
A `Retry-After` header is honored. Change the count with `--retries` or `GYAZO_MAX_RETRIES`; `0` disables retries.
Uploads and deletes are never retried, so they are never applied twice.

## Development

```bash
make test
make vet
make build
```

CI, release and Dependabot are configured the same way as scrapbox-cli: GitHub Actions runs lint, `go test -race` on 3 OSes, govulncheck and a GoReleaser snapshot build, and pushing a `v*` tag publishes binaries to GitHub Releases.

```bash
git tag v0.1.0
git push origin v0.1.0
```
