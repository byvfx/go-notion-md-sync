# Quick Start Guide

## 🚀 5-Minute Setup

### 1. Get Your Notion Integration Token
1. Go to [notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "Create new integration"
3. Copy the token

### 2. Get Your Page ID
1. Open your Notion page
2. Copy the ID from URL: `notion.so/Your-Page-**20e388d7746180eab5d9dd7b9e545e40**`

### 3. Configure Environment
```bash
# Create .env file
cp .env.example .env

# Edit .env with your values:
NOTION_MD_SYNC_NOTION_TOKEN=ntn_your_token_here
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID=your_page_id_here
```

### 4. Build & Test
```bash
make build

# Verify your configuration is ready
./scripts/run-with-env.sh verify

# Test pulling from Notion
./scripts/run-with-env.sh pull --verbose
```

## 📖 Common Commands

```bash
# Verify configuration is ready
./scripts/run-with-env.sh verify

# Check file status (modified, staged, etc.)
./scripts/run-with-env.sh status

# Pull all pages from Notion
./scripts/run-with-env.sh pull --verbose

# Pull a specific page by filename
./scripts/run-with-env.sh pull --page "My Document.md"

# Stage and push files to Notion
./scripts/run-with-env.sh add docs/my-file.md
./scripts/run-with-env.sh push --verbose

# Start auto-sync (watches for file changes)
./scripts/run-with-env.sh watch --verbose

# Sync everything both ways
./scripts/run-with-env.sh sync bidirectional --verbose
```

## 📝 Example Markdown File

```markdown
---
title: "My Document"
sync_enabled: true
---

# My Document

This content will sync with Notion!

## Features
- Headings work
- Lists work
- **Bold** and *italic* text
- Tables work too!

## Data Table

| Feature | Status | Notes |
| --- | --- | --- |
| Headings | ✅ | H1, H2, H3 supported |
| Tables | ✅ | Full bidirectional sync |
| Lists | ✅ | Bullet and numbered |
| Code | ✅ | Syntax highlighting |
```

## 🔧 Troubleshooting

| Problem | Solution |
|---------|----------|
| "notion.token is required" | Check your `.env` file exists and has correct values |
| "Page not found" | Make sure you shared the Notion page with your integration |
| Files not syncing | Add `sync_enabled: true` to frontmatter |

## 📚 Full Documentation

See [README.md](README.md) for complete documentation.