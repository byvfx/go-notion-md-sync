package markdown

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

type Parser interface {
	ParseFile(filePath string) (*Document, error)
	CreateMarkdownWithFrontmatter(filePath string, metadata map[string]interface{}, content string) error
}

type Document struct {
	Metadata map[string]interface{}
	Content  string
	AST      ast.Node
	FilePath string
}

type markdownParser struct {
	md goldmark.Markdown
}

func NewParser() Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)

	return &markdownParser{
		md: md,
	}
}

func (p *markdownParser) ParseFile(filePath string) (*Document, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Parse the markdown with frontmatter
	ctx := parser.NewContext()
	doc := p.md.Parser().Parse(text.NewReader(content), parser.WithContext(ctx))

	// Extract metadata from frontmatter
	metaData := meta.Get(ctx)
	if metaData == nil {
		metaData = make(map[string]interface{})
	}

	// Extract content without frontmatter
	var buf bytes.Buffer
	if err := p.md.Renderer().Render(&buf, content, doc); err != nil {
		return nil, fmt.Errorf("failed to render markdown: %w", err)
	}

	// Get raw content without frontmatter for processing
	rawContent := string(content)
	if metaData != nil && len(metaData) > 0 {
		// Remove frontmatter from raw content
		lines := bytes.Split(content, []byte("\n"))
		if len(lines) > 0 && bytes.Equal(lines[0], []byte("---")) {
			// Find the closing ---
			for i := 1; i < len(lines); i++ {
				if bytes.Equal(lines[i], []byte("---")) {
					if i+1 < len(lines) {
						rawContent = string(bytes.Join(lines[i+1:], []byte("\n")))
					} else {
						rawContent = ""
					}
					break
				}
			}
		}
	}

	return &Document{
		Metadata: metaData,
		Content:  rawContent,
		AST:      doc,
		FilePath: filePath,
	}, nil
}

func (p *markdownParser) CreateMarkdownWithFrontmatter(filePath string, metadata map[string]interface{}, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var buf bytes.Buffer

	// Write frontmatter if metadata exists
	if len(metadata) > 0 {
		buf.WriteString("---\n")

		yamlData, err := yaml.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata to YAML: %w", err)
		}

		buf.Write(yamlData)
		buf.WriteString("---\n\n")
	}

	// Write content
	buf.WriteString(content)

	// Write file
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// Helper functions for AST traversal

func ExtractTextFromAST(node ast.Node, source []byte) string {
	var buf bytes.Buffer

	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch n.Kind() {
			case ast.KindText:
				textNode := n.(*ast.Text)
				buf.Write(textNode.Segment.Value(source))
			case ast.KindString:
				stringNode := n.(*ast.String)
				buf.Write(stringNode.Value)
			}
		}
		return ast.WalkContinue, nil
	})

	return buf.String()
}

func GetHeadings(node ast.Node, source []byte) []Heading {
	var headings []Heading

	ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && n.Kind() == ast.KindHeading {
			heading := n.(*ast.Heading)
			text := ExtractTextFromAST(heading, source)
			headings = append(headings, Heading{
				Level: heading.Level,
				Text:  text,
			})
		}
		return ast.WalkContinue, nil
	})

	return headings
}

type Heading struct {
	Level int
	Text  string
}
