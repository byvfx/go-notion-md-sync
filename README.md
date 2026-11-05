# notion-md-sync

[![CI Status](https://github.com/byvfx/go-notion-md-sync/workflows/CI/badge.svg)](https://github.com/byvfx/go-notion-md-sync/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/byvfx/go-notion-md-sync)](https://goreportcard.com/report/github.com/byvfx/go-notion-md-sync)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release](https://img.shields.io/github/release/byvfx/go-notion-md-sync.svg)](https://github.com/byvfx/go-notion-md-sync/releases)
[![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.23-blue)](https://golang.org)

A command-line tool that syncs a folder of Markdown files with a tree of Notion pages, in either direction.

- **Pull** Notion pages as Markdown. Each page becomes `Page/Page.md`, with sub-pages in sub-folders and databases exported as CSV.
- **Push** your edits back. Only the blocks you changed are touched: edited blocks are updated in place, so block IDs, comments and links survive.
- **Nothing Notion-only is lost.** Sub-pages, databases and blocks Markdown can't express are never deleted. They appear in your file as `<!-- notion:TYPE ID -->` markers.
- **Git-like staging** (`status`, `add`, `reset`), file watching, and a terminal UI.

📖 **Full documentation is in the [wiki](https://github.com/byvfx/go-notion-md-sync/wiki).**

## Install

**Windows (PowerShell):**

```powershell
iwr -useb https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-windows.ps1 | iex
```

**Linux / macOS:**

```bash
curl -sSL https://raw.githubusercontent.com/byvfx/go-notion-md-sync/main/scripts/install-unix.sh | bash
```

Or download a binary from [Releases](https://github.com/byvfx/go-notion-md-sync/releases), or build from source with Go 1.23+ (`make build`). See [Installation](https://github.com/byvfx/go-notion-md-sync/wiki/Installation).

## Quick start

1. Create an integration at [notion.so/my-integrations](https://www.notion.so/my-integrations) and copy its token.
2. Open the Notion page you want to sync under. In its `•••` menu, go to **Connections** and add the integration. Copy the page ID from the URL.
3. In your project folder, run the guided setup:

   ```bash
   notion-md-sync init
   ```

   Or set things up by hand. Put the token and page ID in a `.env` file (don't commit it):

   ```bash
   NOTION_MD_SYNC_NOTION_TOKEN=ntn_your_token
   NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_page_id
   ```

   and create a `config.yaml`:

   ```yaml
   directories:
     markdown_root: ./docs
   ```

4. Check the setup, then pull:

   ```bash
   notion-md-sync verify
   notion-md-sync pull
   ```

More options are in [Configuration](https://github.com/byvfx/go-notion-md-sync/wiki/Configuration).

## Core commands

```bash
notion-md-sync pull                    # Notion → Markdown, everything under the parent page
notion-md-sync status                  # what changed locally
notion-md-sync add docs/Page/Page.md   # stage files (`add .` stages everything)
notion-md-sync push                    # push staged files
notion-md-sync push docs/Page/Page.md  # stage and push one file
notion-md-sync sync bidirectional      # pull and push, resolving conflicts
notion-md-sync watch                   # push files as you save them
notion-md-sync tui                     # terminal UI
```

Add `--dry-run` to `pull`, `push` or `sync` to see what would happen without changing anything, and `--verbose` for more output. Every flag is described in [Commands](https://github.com/byvfx/go-notion-md-sync/wiki/Commands).

## Documentation

| Wiki page | Covers |
| --- | --- |
| [Installation](https://github.com/byvfx/go-notion-md-sync/wiki/Installation) | Install scripts, binaries, source builds, shell completion |
| [Configuration](https://github.com/byvfx/go-notion-md-sync/wiki/Configuration) | `init`, `config.yaml`, `.env`, environment variables, every setting |
| [Commands](https://github.com/byvfx/go-notion-md-sync/wiki/Commands) | Every command and flag, TUI keys |
| [Directory Layout and Kept Blocks](https://github.com/byvfx/go-notion-md-sync/wiki/Directory-Layout-and-Kept-Blocks) | How files map to pages, marker lines, how push edits a page |
| [Markdown Support](https://github.com/byvfx/go-notion-md-sync/wiki/Markdown-Support) | Frontmatter and supported syntax |
| [Databases](https://github.com/byvfx/go-notion-md-sync/wiki/Databases) | CSV export |
| [Troubleshooting](https://github.com/byvfx/go-notion-md-sync/wiki/Troubleshooting) | Common errors and known quirks |
| [Development](https://github.com/byvfx/go-notion-md-sync/wiki/Development) | Building, testing, releasing |

Release notes are in [`docs/releases/`](docs/releases/).

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Development](https://github.com/byvfx/go-notion-md-sync/wiki/Development) wiki page.

## License

MIT. See [LICENSE](LICENSE).
