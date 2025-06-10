# CLAUDE_go.md - Guidelines for notion-md-sync (Go Implementation)

## Commands
- Build: `go build -o notion-md-sync ./cmd/notion-md-sync`
- Dev/Run: `go run ./cmd/notion-md-sync`
- Test: `go test ./...`
- Single test: `go test -run TestName ./pkg/package`
- Lint: `golangci-lint run`
- Format: `go fmt ./...`
- Mod tidy: `go mod tidy`

## Code Style
- **Formatting**: Use `gofmt` and `goimports`
- **Naming**: Follow Go conventions (PascalCase for exported, camelCase for unexported)
- **Packages**: Short, lowercase names without underscores
- **Error Handling**: Always check and handle errors explicitly
- **Interfaces**: Keep them small and focused
- **Documentation**: Use godoc comments for exported functions
- **Tests**: Use table-driven tests where appropriate

## Project Structure
```
notion-md-sync/
├── cmd/
│   └── notion-md-sync/           # Main application entry point
│       └── main.go
├── pkg/
│   ├── config/                   # Configuration management
│   │   ├── config.go
│   │   └── config_test.go
│   ├── notion/                   # Notion API client
│   │   ├── client.go
│   │   ├── client_test.go
│   │   └── types.go
│   ├── markdown/                 # Markdown processing
│   │   ├── parser.go
│   │   ├── parser_test.go
│   │   └── frontmatter.go
│   ├── sync/                     # Core sync logic
│   │   ├── engine.go
│   │   ├── engine_test.go
│   │   └── converter.go
│   ├── watcher/                  # File system monitoring
│   │   ├── watcher.go
│   │   └── watcher_test.go
│   └── cli/                      # Command line interface
│       ├── root.go
│       ├── sync.go
│       ├── pull.go
│       └── watch.go
├── internal/                     # Private application code
│   └── util/                     # Internal utilities
├── configs/
│   └── config.example.yaml
├── go.mod
├── go.sum
├── README.md
└── Makefile
```

## Dependencies (go.mod)
```go
module github.com/yourusername/notion-md-sync

go 1.21 or above

require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    github.com/fsnotify/fsnotify v1.7.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/yuin/goldmark v1.6.0
    github.com/yuin/goldmark-meta v1.1.0
    github.com/stretchr/testify v1.8.4
)
```

## Key Go Packages to Use

### CLI Framework
- **cobra**: Command-line interface framework
- **viper**: Configuration management with YAML/ENV support

### HTTP Client
- **net/http**: Built-in HTTP client for Notion API
- **context**: For request timeouts and cancellation

### Markdown Processing
- **goldmark**: Extensible markdown parser
- **goldmark-meta**: Frontmatter support for goldmark

### File Watching
- **fsnotify**: Cross-platform filesystem notification

### YAML Processing
- **gopkg.in/yaml.v3**: YAML parsing for frontmatter and config

### Testing
- **testify**: Assertions and mocking framework

## Core Types and Interfaces

```go
// pkg/config/config.go
type Config struct {
    Notion struct {
        Token        string `yaml:"token"`
        ParentPageID string `yaml:"parent_page_id"`
    } `yaml:"notion"`
    
    Sync struct {
        Direction           string `yaml:"direction"`
        ConflictResolution  string `yaml:"conflict_resolution"`
    } `yaml:"sync"`
    
    Directories struct {
        MarkdownRoot     string   `yaml:"markdown_root"`
        ExcludedPatterns []string `yaml:"excluded_patterns"`
    } `yaml:"directories"`
    
    Mapping struct {
        Strategy string `yaml:"strategy"`
    } `yaml:"mapping"`
}

// pkg/notion/types.go
type Page struct {
    ID         string                 `json:"id"`
    Object     string                 `json:"object"`
    CreatedBy  User                   `json:"created_by"`
    Properties map[string]interface{} `json:"properties"`
    URL        string                 `json:"url"`
}

type Block struct {
    ID       string                 `json:"id"`
    Object   string                 `json:"object"`
    Type     string                 `json:"type"`
    HasChildren bool                `json:"has_children"`
    // Dynamic content based on type
}

// pkg/notion/client.go
type Client interface {
    GetPage(ctx context.Context, pageID string) (*Page, error)
    GetPageBlocks(ctx context.Context, pageID string) ([]Block, error)
    CreatePage(ctx context.Context, parentID string, properties map[string]interface{}) (*Page, error)
    UpdatePageBlocks(ctx context.Context, pageID string, blocks []Block) error
    SearchPages(ctx context.Context, query string) ([]Page, error)
    GetChildPages(ctx context.Context, parentID string) ([]Page, error)
}

// pkg/markdown/parser.go
type Parser interface {
    ParseFile(filePath string) (*Document, error)
    CreateMarkdownWithFrontmatter(filePath string, metadata map[string]interface{}, content string) error
}

type Document struct {
    Metadata map[string]interface{}
    Content  string
    AST      interface{} // goldmark AST
}

// pkg/sync/engine.go
type Engine interface {
    SyncFileToNotion(ctx context.Context, filePath string) error
    SyncNotionToFile(ctx context.Context, pageID, filePath string) error
    SyncAll(ctx context.Context, direction string) error
}
```

## Implementation Strategy

### Phase 1: Foundation
1. **Project Setup**
   ```bash
   go mod init github.com/yourusername/notion-md-sync
   mkdir -p cmd/notion-md-sync pkg/{config,notion,markdown,sync,watcher,cli}
   ```

2. **Configuration System**
   ```go
   // Use viper for config loading with YAML and ENV support
   func LoadConfig(configPath string) (*Config, error) {
       viper.SetConfigFile(configPath)
       viper.AutomaticEnv()
       viper.SetEnvPrefix("NOTION_MD_SYNC")
       // Implementation
   }
   ```

3. **Notion API Client**
   ```go
   type notionClient struct {
       httpClient *http.Client
       token      string
       baseURL    string
   }
   
   func (c *notionClient) GetPage(ctx context.Context, pageID string) (*Page, error) {
       req, err := http.NewRequestWithContext(ctx, "GET", 
           fmt.Sprintf("%s/pages/%s", c.baseURL, pageID), nil)
       if err != nil {
           return nil, err
       }
       
       req.Header.Set("Authorization", "Bearer "+c.token)
       req.Header.Set("Notion-Version", "2022-06-28")
       // Implementation
   }
   ```

### Phase 2: Core Functionality
1. **Markdown Parser with Frontmatter**
   ```go
   func (p *parser) ParseFile(filePath string) (*Document, error) {
       content, err := os.ReadFile(filePath)
       if err != nil {
           return nil, err
       }
       
       md := goldmark.New(
           goldmark.WithExtensions(meta.Meta),
       )
       
       var buf bytes.Buffer
       ctx := parser.NewContext()
       if err := md.Convert(content, &buf, parser.WithContext(ctx)); err != nil {
           return nil, err
       }
       
       metaData := meta.Get(ctx)
       // Implementation
   }
   ```

2. **Block Converter**
   ```go
   func (c *converter) MarkdownToBlocks(content string) ([]Block, error) {
       md := goldmark.New()
       doc := md.Parser().Parse(text.NewReader([]byte(content)))
       
       var blocks []Block
       ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
           if entering {
               switch n.Kind() {
               case ast.KindHeading:
                   // Convert heading to Notion block
               case ast.KindParagraph:
                   // Convert paragraph to Notion block
               }
           }
           return ast.WalkContinue, nil
       })
       return blocks, nil
   }
   ```

### Phase 3: CLI Interface
```go
// pkg/cli/root.go
var rootCmd = &cobra.Command{
    Use:   "notion-md-sync",
    Short: "Bridge between markdown files and Notion pages",
}

// pkg/cli/sync.go  
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Sync a single file between markdown and Notion",
    RunE: func(cmd *cobra.Command, args []string) error {
        filePath, _ := cmd.Flags().GetString("file")
        direction, _ := cmd.Flags().GetString("direction")
        
        cfg, err := config.Load(configPath)
        if err != nil {
            return err
        }
        
        engine := sync.NewEngine(cfg)
        return engine.SyncFile(cmd.Context(), filePath, direction)
    },
}

func init() {
    syncCmd.Flags().StringP("file", "f", "", "Markdown file to sync")
    syncCmd.Flags().StringP("direction", "d", "push", "Sync direction")
    syncCmd.MarkFlagRequired("file")
    rootCmd.AddCommand(syncCmd)
}
```

### Phase 4: File Watching
```go
// pkg/watcher/watcher.go
type Watcher struct {
    fsWatcher *fsnotify.Watcher
    engine    sync.Engine
    config    *config.Config
}

func (w *Watcher) Start(ctx context.Context) error {
    defer w.fsWatcher.Close()
    
    for {
        select {
        case event, ok := <-w.fsWatcher.Events:
            if !ok {
                return nil
            }
            if event.Op&fsnotify.Write == fsnotify.Write {
                if strings.HasSuffix(event.Name, ".md") {
                    go w.handleFileChange(ctx, event.Name)
                }
            }
        case err, ok := <-w.fsWatcher.Errors:
            if !ok {
                return nil
            }
            return err
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

## Error Handling Patterns

```go
// Custom error types
type NotionAPIError struct {
    Code    int
    Message string
    PageID  string
}

func (e *NotionAPIError) Error() string {
    return fmt.Sprintf("notion api error %d: %s (page: %s)", e.Code, e.Message, e.PageID)
}

// Error wrapping
func (c *client) GetPage(ctx context.Context, pageID string) (*Page, error) {
    resp, err := c.doRequest(ctx, "GET", "/pages/"+pageID, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to get page %s: %w", pageID, err)
    }
    // Implementation
}
```

## Testing Strategy

```go
// Table-driven tests
func TestConverter_MarkdownToBlocks(t *testing.T) {
    tests := []struct {
        name     string
        markdown string
        want     []Block
        wantErr  bool
    }{
        {
            name:     "simple heading",
            markdown: "# Hello World",
            want: []Block{
                {Type: "heading_1", Content: "Hello World"},
            },
            wantErr: false,
        },
        // More test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c := NewConverter()
            got, err := c.MarkdownToBlocks(tt.markdown)
            if (err != nil) != tt.wantErr {
                t.Errorf("MarkdownToBlocks() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            assert.Equal(t, tt.want, got)
        })
    }
}

// Mock interfaces
type mockNotionClient struct {
    pages map[string]*Page
    blocks map[string][]Block
}

func (m *mockNotionClient) GetPage(ctx context.Context, pageID string) (*Page, error) {
    if page, exists := m.pages[pageID]; exists {
        return page, nil
    }
    return nil, &NotionAPIError{Code: 404, Message: "Page not found", PageID: pageID}
}
```

## Makefile

```makefile
.PHONY: build test lint clean install

# Binary name
BINARY_NAME=notion-md-sync

# Build the application
build:
	go build -o bin/$(BINARY_NAME) ./cmd/notion-md-sync

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Lint the code
lint:
	golangci-lint run

# Format code
fmt:
	go fmt ./...
	goimports -w .

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install the binary
install:
	go install ./cmd/notion-md-sync

# Run the application
run:
	go run ./cmd/notion-md-sync

# Development setup
dev-setup:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
```

## Key Differences from Python Implementation

1. **Static Typing**: All types defined at compile time
2. **Error Handling**: Explicit error returns vs exceptions
3. **Concurrency**: Goroutines for file watching and concurrent operations
4. **Package Management**: Go modules vs pip requirements
5. **Binary Distribution**: Single compiled binary vs Python runtime requirement
6. **Performance**: Generally faster execution and lower memory usage
7. **Deployment**: Simple binary deployment vs Python environment setup

## Development Workflow

1. Start with interfaces and types
2. Implement core packages (config, notion client)
3. Add markdown processing with goldmark
4. Build sync engine with proper error handling
5. Create CLI with cobra
6. Add file watching with fsnotify
7. Write comprehensive tests
8. Add CI/CD pipeline
9. Create installation scripts and documentation

This Go implementation will provide better performance, easier deployment, and type safety while maintaining the same functionality as the Python version.
