---
name: gy
description: Upload, find, read and download Gyazo images with the gy CLI. Use when the user wants to share a screenshot or image as a URL, put an image into a Scrapbox (Cosense) page or Markdown, find a past screenshot by the text in it, read the text in a Gyazo image, or back up or delete Gyazo images. Also use when a gyazo.com or i.gyazo.com URL appears and its image or text is needed.
---

# gy — Gyazo CLI

`gy` calls the official Gyazo API. Read `gy --help` and `gy <command> --help` when a detail is not covered here; the help states every output format.

## Before the first command

Run `gy auth status`. If it says "not configured" or fails with 401, stop and ask the user to run `gy auth login` themselves (or set `GYAZO_ACCESS_TOKEN`). Never ask the user to paste the token into the conversation, and never pass a token as an argument.

## Rules

- Use `-o json` whenever you parse the output. Results are on stdout; counts, hints and errors are on stderr. Exit code is 0 on success and 1 on any error.
- An image argument `<id|url>` accepts an image ID or any Gyazo URL (`https://gyazo.com/<id>`, `https://i.gyazo.com/<id>.png`, `https://<team>.gyazo.com/<id>`). Pass URLs as-is; do not extract the ID yourself.
- **Screenshots may contain secrets.** Upload with `--access-policy only_me` unless the user wants a shareable link or the image is clearly public. Say which policy you used.
- **Never delete without the user's explicit approval of the specific images.** Run `gy image delete <id|url>...` without `--yes` first, show the result, and add `--yes` only after the user confirms.
- **Never re-run an upload after an unclear failure** (timeout, network error). Uploads are not idempotent. First run `gy image list --per-page 5 -o json` and check whether the image already arrived.
- The API allows 12,500 requests per day. Avoid `gy image list --all` unless the user needs every image; it sends one request per 100 images.

## Commands

| Task | Command |
|---|---|
| Upload files or stdin | `gy upload <file\|->... [--title T] [--desc D] [--access-policy anyone\|only_me]` |
| Choose the printed link | `--as permalink\|url\|markdown\|scrapbox` (upload, image list, search) |
| List recent images | `gy image list [--page N] [--per-page N]` (max 100 per page) |
| Show URLs and capture metadata | `gy image get <id\|url>` |
| Read the text in an image (OCR) | `gy image ocr <id\|url>` |
| Save image files | `gy image download <id\|url>... [--dir D] [--force]`, or `--stdout` for one image |
| Search by text (Gyazo Pro only) | `gy search <query>... [--per N] [--page N]` |
| Delete (needs approval, see Rules) | `gy image delete <id\|url>...`, then add `--yes` |
| Current user | `gy whoami` |

## JSON output shapes

Field names follow the Gyazo API.

| Command | Shape |
|---|---|
| `upload` | `[{image_id, permalink_url, thumb_url, url, type}]` (always an array) |
| `image list` | `{total_count, page, per_page, user_type, images: [Image]}` |
| `image get` | `Image` |
| `image ocr` | `{locale, description}` |
| `image download` | `[{image_id, path, bytes}]` |
| `image delete --yes` | `[{image_id, type}]` |
| `search` | `{query, page, per, images: [Image]}` |

`Image` is `{image_id, permalink_url, thumb_url, url, type, created_at, metadata: {app, title, url, desc}, ocr: {locale, description}}`. `metadata` and `ocr` can be missing. Use `permalink_url` for the image page and `url` for the image file.

## Workflows

**Share a screenshot as a link**

```bash
gy upload shot.png --access-policy only_me          # prints https://gyazo.com/<id>
```

**Put an image into a Scrapbox page** (with the sibling `sb` CLI)

```bash
link=$(gy upload diagram.png --as scrapbox)          # [https://gyazo.com/<id>]
sb page append "Design notes" -t "$link"
```

For Markdown, use `--as markdown` (`![](https://i.gyazo.com/<id>.png)`).

**Find a past screenshot by its content**

```bash
gy search "connection refused" -o json | jq -r '.images[] | "\(.permalink_url)\t\(.metadata.title // "")"'
```

If search fails with 402, the user has no Pro plan. Fall back to scanning recent images and their OCR text, and tell the user it only covers the images you scanned:

```bash
gy image list --per-page 50 -o json | jq -r '.images[].image_id' |
  while read -r id; do printf '%s\t' "$id"; gy image ocr "$id" | tr '\n' ' '; echo; done
```

Each `image ocr` is one request; keep the scan small.

**Read the text in an image the user linked**

```bash
gy image ocr https://gyazo.com/<id>
```

If it prints nothing (stderr says "no text recognized"), download the image and look at it instead:

```bash
gy image download https://gyazo.com/<id> --dir /tmp/gy
```

**Back up images**

```bash
gy image list --per-page 100 --as permalink | xargs gy image download --dir backup
```

The command stops at the first file that already exists (`--force` overwrites). For a second run, use a new `--dir`. Downloads write to a temp file first, so an interrupted run leaves no broken files.

## Errors

The error is on stderr (`error: <METHOD> <URL>: <status> <message>`), followed by a `hint:` line for common statuses. Act on the HTTP status:

| Status | Meaning | What to do |
|---|---|---|
| 401 | No token or invalid token | Ask the user to run `gy auth login` |
| 402 | Needs Gyazo Pro (search) | Tell the user; use the OCR fallback above if useful |
| 403 | Not the user's image | Only the user's own images can be read in detail or deleted |
| 404 | No such image | Check the ID or URL |
| 422 | Invalid value (e.g. search beyond 10,000 results) | Fix the arguments; do not retry as-is |
| 429 | Daily limit reached | Stop and tell the user; do not loop |

`gy` already retries GET requests on 429 and 5xx (3 times with backoff). Do not add your own retry loop.

## Not possible with the public API

Editing an image after upload (title, description, tags), managing collections (`--collection` only adds a new upload), and listing other users' images. Say so instead of trying workarounds such as browser automation.
