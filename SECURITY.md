# Security Configuration

## Environment Variables

For security, sensitive configuration values should be set via environment variables rather than stored in config files.

### Required Environment Variables

```bash
# Your Notion integration token
export NOTION_MD_SYNC_NOTION_TOKEN="your_token_here"

# Your Notion parent page ID
export NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID="your_page_id_here"
```

### Setup Methods

#### Method 1: Using .env file (recommended for development)

1. Copy your secrets to `.env`:
```bash
cp .env.example .env
# Edit .env with your actual values
```

2. Run with environment variables:
```bash
make run-env
# or
./scripts/run-with-env.sh --help
```

#### Method 2: Export in your shell

```bash
export NOTION_MD_SYNC_NOTION_TOKEN="your_token_here"
export NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID="your_page_id_here"
./bin/notion-md-sync --help
```

#### Method 3: One-time execution

```bash
NOTION_MD_SYNC_NOTION_TOKEN="your_token" \
NOTION_MD_SYNC_NOTION_PARENT_PAGE_ID="your_page_id" \
./bin/notion-md-sync sync push
```

### Production Deployment

For production environments:

1. **Never commit `.env` files or config files with secrets**
2. Use your platform's secret management:
   - Docker: `--env-file` or `-e` flags
   - Kubernetes: Secrets and ConfigMaps
   - Cloud platforms: Built-in secret managers
3. Use the `config.template.yaml` as a base for your configuration

### Obtaining Your Notion Token

1. Go to [https://www.notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Create a new integration
3. Copy the "Internal Integration Token"
4. Share your Notion page with the integration

### Finding Your Parent Page ID

From a Notion page URL like:
`https://www.notion.so/Your-Page-20e388d7746180eab5d9dd7b9e545e40`

The page ID is: `20e388d7746180eab5d9dd7b9e545e40`