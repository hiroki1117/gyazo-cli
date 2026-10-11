# gyazo-cli

`gy` is a command line client for uploading and managing [Gyazo](https://gyazo.com) images, written in Go.
It is a sibling of [scrapbox-cli](https://github.com/hiroki1117/scrapbox-cli) (`sb`) and shares its conventions.

## Features

- Upload images from files or standard input, and print links as a permalink, image URL, Markdown or Scrapbox notation
- List, show and delete your images, and print the text recognized by OCR
- Full-text search of your images, including OCR text and metadata (Gyazo Pro)
- Download image files without leaving partial files or overwriting existing ones
- Print results as JSON with `-o json` for `jq` and AI agents

## Install

| Method | Steps |
|---|---|
| Binary | Download the archive for your OS from [Releases](https://github.com/hiroki1117/gyazo-cli/releases) and put `gy` on your PATH |
| go install | `go install github.com/hiroki1117/gyazo-cli/cmd/gy@latest` (Go 1.27+) |
| From source | `git clone`, then `make build` (creates `bin/gy`) |

Binaries are available for Linux, macOS and Windows on amd64 and arm64. Run `gy version` to check the version.
To verify that an archive was built by this repository's release workflow, run `gh attestation verify <archive> -R hiroki1117/gyazo-cli`.

## Quick start

```bash
gy auth login                      # 1. Save your access token
gy upload screenshot.png           # 2. Upload an image and print its URL
gy image list                      # 3. List your images
gy search keyword                  # Search your images (Gyazo Pro)
```

Run `gy --help` for the list of commands and `gy <command> --help` for details.

## Configuration

Settings come from flags, environment variables and the config file. The precedence is **flags > environment variables > config file**.

### Authentication

Every API request needs an OAuth access token.

1. Open [Applications](https://gyazo.com/oauth/applications) and register an application (any name; the callback URL is not used)
2. Open the application and click "Your Access Token"
3. Save it with `gy auth login`, or set `GYAZO_ACCESS_TOKEN`

Use `gy auth login` to save the token in the config file. The input is hidden.

```bash
gy auth login                          # Enter the token at the prompt
pbpaste | gy auth login --with-token   # Read from stdin
gy auth status                         # Show the current auth state (token masked)
gy auth logout                         # Remove from the config file
```

- `login` verifies the token against the API before saving. Use `--no-verify` to skip this.
- Tokens are never accepted as command line arguments, to keep them out of shell history.
- If `GYAZO_ACCESS_TOKEN` is set, the token in the config file is ignored.

### Config file

The default path is `~/.config/gy/config.json` (change it with `$XDG_CONFIG_HOME/gy/config.json` or `GY_CONFIG`).
It contains the access token, so it is created with permission 0600.

```bash
gy config path                               # Print the path
gy config get upload.access-policy
gy config set upload.access-policy only_me   # Make uploads private by default
gy config set upload.app my-script           # Application name recorded with uploads
gy config unset upload.app
```

### Environment variables

| Variable | Flag | Description |
|---|---|---|
| `GYAZO_ACCESS_TOKEN` | - | OAuth access token |
| `GYAZO_API_URL` | - | API base URL (default: `https://api.gyazo.com`) |
| `GYAZO_UPLOAD_URL` | - | Upload API base URL (default: `https://upload.gyazo.com`) |
| `GYAZO_TIMEOUT` | `--timeout` | Request timeout (default: `30s`) |
| `GYAZO_MAX_RETRIES` | `--retries` | Retries on transient errors (default: `3`) |
| `GY_CONFIG` | - | Config file path |

## Usage

Commands that take an image accept an image ID or a Gyazo URL (`https://gyazo.com/<id>`, `https://i.gyazo.com/<id>.png` or `https://<team>.gyazo.com/<id>`).

```bash
# Upload (files, or "-" for stdin)
gy upload screenshot.png                    # Print the permalink
gy upload *.png --as scrapbox | pbcopy      # Link style: permalink|url|markdown|scrapbox
pngpaste - | gy upload -                    # From the clipboard (macOS, requires pngpaste)
gy upload shot.png --title "Design review" --desc "#ui" --access-policy only_me
# The public API cannot edit an image after upload. --collection only adds a new upload to a collection.

# Images
gy image list                               # Newest first (--page, --per-page)
gy image list --all -o json                 # Fetch all images, paging automatically
gy image list --per-page 5 --as markdown    # One link per line instead of a table
gy image get <id|url>                       # URLs and capture metadata
gy image ocr <id|url>                       # Text recognized by OCR
gy image download <id|url>... --dir backup  # Save as <id>.<ext> (--force to overwrite)
gy image download <id|url> --stdout > a.png
gy image delete <id|url>...                 # Show the images to delete; add --yes to delete

# Search (requires Gyazo Pro; up to 200 characters, only the first 10,000 results)
gy search invoice                           # Full-text search, including OCR text
gy search invoice 2026 --per 50 --page 2

# Users
gy whoami

# Other
gy version
gy help environment                         # Environment variables and precedence
gy completion zsh > "${fpath[1]}/_gy"       # Shell completion (bash / zsh / fish / powershell)
```

### Output format

`-o json` prints JSON that keeps the API field names (`image_id`, `permalink_url`, ...). It is meant to be used with `jq`.

```bash
gy image list --all -o json | jq -r '.images[] | select(.metadata.app == "Chrome") | .permalink_url'
```

`image download --stdout` always prints the raw image data regardless of `-o`, so it can be redirected to a file.

## Design

```
cmd/gy/            Entry point (signals, exit code, error hints)
internal/
  cli/             cobra commands. App passes the config, Client and Printer to each command
  config/          Loads the config file and environment variables; saves the config file
  output/          Text / JSON output, table and time formatting
  gyazo/           REST API client (auth, retries, error conversion, APIs)
```

Dependencies flow `cmd → cli → (config, output, gyazo)`. The `gyazo` package depends on no other package in this module.

### Extending

- **Add an API**: Add a method to the matching service in `gyazo` (such as `ImagesService`). For a new resource, define `type XxxService service` and register it in `NewClient`. Use `getJSON`, or `newAPIRequest` and `doJSON`, for the common HTTP handling.
- **Add a command**: Define `newXxxCmd(app *App) *cobra.Command` in `internal/cli` and add it to its parent with `AddCommand`. Print with `app.Printer().Print(v, textFunc)` to support `-o json` automatically. Write `Long` (including the output format) and `Example` (`help_test.go` checks this for image commands).
- **Add a link style**: Add a function to `linkStyles` in `internal/cli/link.go`. It becomes available to `--as` of every command.
- **Add an auth method**: Implement `gyazo.Authenticator` and return it from `config.Credentials.Authenticator`.
- **Add a setting**: Add a field to `config.File` and register it in `configKeys` in `internal/cli/configcmd.go` (with `validate` if the value is restricted).

### Help

`gy --help` covers how to get started and the common rules (argument format, output streams, exit codes). Each command's `--help` covers its output format and examples.
For AI agents, start by having them read `gy --help`.

### Uploads

The file is streamed with an exact `Content-Length`, so large files are not read into memory. Only standard input is buffered, because its size is unknown.
The content type is detected from the data, not from the file extension.

### Retries

429 / 502 / 503 / 504 and network errors are retried up to 3 times with exponential backoff (with jitter).
A `Retry-After` header is honored. Change the count with `--retries` or `GYAZO_MAX_RETRIES`; `0` disables retries.
Uploads and deletes are never retried, so they are never applied twice.
The API allows 12,500 requests per day; `image list --all` sends one request per 100 images.

## Development

```bash
make test
make vet
make build
```

### CI

GitHub Actions (`.github/workflows/ci.yml`) runs the following on every push and pull request, and weekly.

| Job | What it does |
|---|---|
| lint | `gofmt` diff, `go mod tidy` diff, `go vet`, golangci-lint |
| test | `go test -race` on ubuntu / macos / windows × Go 1.27 (minimum) and stable |
| vulncheck | govulncheck checks dependencies and the standard library for known vulnerabilities |
| release-check | GoReleaser builds all targets to verify the release config (nothing is published) |

Dependabot opens weekly pull requests to update Go modules and GitHub Actions.

### Release

Pushing a tag starting with `v` runs `.github/workflows/release.yml`, which uses GoReleaser (`.goreleaser.yaml`) to build binaries and checksums for each OS, attest their build provenance, and publish them to GitHub Releases.
Only users with write access to this repository can push tags. All actions are pinned by commit SHA.

```bash
git tag v0.1.0
git push origin v0.1.0
make snapshot   # Build the same artifacts into dist/ without publishing (requires goreleaser)
```
