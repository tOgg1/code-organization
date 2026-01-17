package chunker

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/bash"
	clang "github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/csharp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterChunker extracts semantic code chunks using tree-sitter AST parsing
type TreeSitterChunker struct {
	config  Config
	parsers map[string]*sitter.Parser
}

// NewTreeSitter creates a new tree-sitter based chunker
func NewTreeSitter(cfg Config) *TreeSitterChunker {
	if cfg.MaxChunkLines == 0 {
		cfg.MaxChunkLines = 100
	}
	if cfg.MinChunkLines == 0 {
		cfg.MinChunkLines = 5
	}
	if cfg.OverlapLines == 0 {
		cfg.OverlapLines = 3
	}

	return &TreeSitterChunker{
		config:  cfg,
		parsers: make(map[string]*sitter.Parser),
	}
}

// getParser returns a parser for the given language, creating it if necessary
func (c *TreeSitterChunker) getParser(lang string) (*sitter.Parser, *sitter.Language, error) {
	var tsLang *sitter.Language

	switch lang {
	case "go":
		tsLang = golang.GetLanguage()
	case "python":
		tsLang = python.GetLanguage()
	case "javascript":
		tsLang = javascript.GetLanguage()
	case "typescript":
		tsLang = typescript.GetLanguage()
	case "rust":
		tsLang = rust.GetLanguage()
	case "ruby":
		tsLang = ruby.GetLanguage()
	case "java":
		tsLang = java.GetLanguage()
	case "c":
		tsLang = clang.GetLanguage()
	case "cpp":
		tsLang = cpp.GetLanguage()
	case "csharp":
		tsLang = csharp.GetLanguage()
	case "bash":
		tsLang = bash.GetLanguage()
	default:
		return nil, nil, fmt.Errorf("unsupported language: %s", lang)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(tsLang)
	return parser, tsLang, nil
}

// SupportedLanguages returns the list of supported languages
func (c *TreeSitterChunker) SupportedLanguages() []string {
	return []string{
		"go", "python", "javascript", "typescript", "rust",
		"ruby", "java", "c", "cpp", "csharp", "bash",
	}
}

// Chunk extracts semantic code chunks from source code
func (c *TreeSitterChunker) Chunk(source []byte, filename string) ([]Chunk, error) {
	lang := DetectLanguage(filename)
	if lang == "" {
		return nil, fmt.Errorf("unsupported file type: %s", filename)
	}

	parser, tsLang, err := c.getParser(lang)
	if err != nil {
		// Fall back to line-based chunking for unsupported languages
		return c.chunkByLines(source, lang)
	}
	defer parser.Close()

	// Parse the source code
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("parsing source: %w", err)
	}
	defer tree.Close()

	// Extract semantic chunks
	chunks := c.extractChunks(tree.RootNode(), source, lang, tsLang)

	// If no semantic chunks found, fall back to line-based chunking
	if len(chunks) == 0 {
		return c.chunkByLines(source, lang)
	}

	return chunks, nil
}

// extractChunks recursively extracts semantic chunks from the AST
func (c *TreeSitterChunker) extractChunks(node *sitter.Node, source []byte, lang string, tsLang *sitter.Language) []Chunk {
	var chunks []Chunk

	// Get interesting node types for this language
	interestingTypes := c.getInterestingNodeTypes(lang)

	// Walk the AST
	c.walkAST(node, source, lang, interestingTypes, &chunks)

	// Post-process: merge small adjacent chunks, split large ones
	chunks = c.postProcess(chunks, source)

	return chunks
}

// getInterestingNodeTypes returns AST node types that represent semantic units
func (c *TreeSitterChunker) getInterestingNodeTypes(lang string) map[string]string {
	// Maps AST node type to chunk type
	switch lang {
	case "go":
		return map[string]string{
			"function_declaration": "function",
			"method_declaration":   "method",
			"type_declaration":     "type",
			"type_spec":            "type",
			"const_declaration":    "const",
			"var_declaration":      "var",
		}
	case "python":
		return map[string]string{
			"function_definition":  "function",
			"class_definition":     "class",
			"decorated_definition": "decorated",
		}
	case "javascript", "typescript":
		return map[string]string{
			"function_declaration":   "function",
			"function_expression":    "function",
			"arrow_function":         "function",
			"method_definition":      "method",
			"class_declaration":      "class",
			"export_statement":       "export",
			"lexical_declaration":    "const", // const/let
			"variable_declaration":   "var",
			"interface_declaration":  "interface",
			"type_alias_declaration": "type",
			"enum_declaration":       "enum",
		}
	case "rust":
		return map[string]string{
			"function_item":    "function",
			"impl_item":        "impl",
			"struct_item":      "struct",
			"enum_item":        "enum",
			"trait_item":       "trait",
			"mod_item":         "module",
			"const_item":       "const",
			"static_item":      "static",
			"type_item":        "type",
			"macro_definition": "macro",
		}
	case "java":
		return map[string]string{
			"method_declaration":      "method",
			"constructor_declaration": "constructor",
			"class_declaration":       "class",
			"interface_declaration":   "interface",
			"enum_declaration":        "enum",
			"field_declaration":       "field",
		}
	case "ruby":
		return map[string]string{
			"method":           "method",
			"singleton_method": "method",
			"class":            "class",
			"module":           "module",
		}
	case "c", "cpp":
		return map[string]string{
			"function_definition":  "function",
			"struct_specifier":     "struct",
			"class_specifier":      "class",
			"enum_specifier":       "enum",
			"namespace_definition": "namespace",
		}
	case "csharp":
		return map[string]string{
			"method_declaration":      "method",
			"constructor_declaration": "constructor",
			"class_declaration":       "class",
			"interface_declaration":   "interface",
			"struct_declaration":      "struct",
			"enum_declaration":        "enum",
			"property_declaration":    "property",
		}
	default:
		return map[string]string{}
	}
}

// walkAST traverses the AST and collects semantic chunks
func (c *TreeSitterChunker) walkAST(node *sitter.Node, source []byte, lang string, interestingTypes map[string]string, chunks *[]Chunk) {
	nodeType := node.Type()

	if chunkType, isInteresting := interestingTypes[nodeType]; isInteresting {
		startLine := int(node.StartPoint().Row) + 1 // 1-indexed
		endLine := int(node.EndPoint().Row) + 1

		// Get the content
		content := string(source[node.StartByte():node.EndByte()])

		// Try to extract symbol name
		symbolName := c.extractSymbolName(node, source, lang)

		chunk := Chunk{
			StartLine:     startLine,
			EndLine:       endLine,
			Content:       content,
			ChunkType:     chunkType,
			SymbolName:    symbolName,
			Language:      lang,
			TokenEstimate: EstimateTokens(content),
		}

		*chunks = append(*chunks, chunk)

		// Don't recurse into children of interesting nodes
		// (they're part of this chunk)
		return
	}

	// Recurse into children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		c.walkAST(child, source, lang, interestingTypes, chunks)
	}
}

// extractSymbolName tries to extract the name of a symbol from an AST node
func (c *TreeSitterChunker) extractSymbolName(node *sitter.Node, source []byte, lang string) string {
	// Try to find identifier/name child nodes
	nameNodeTypes := []string{"identifier", "name", "property_identifier", "type_identifier"}

	for _, nameType := range nameNodeTypes {
		nameNode := node.ChildByFieldName(nameType)
		if nameNode != nil {
			return nameNode.Content(source)
		}
	}

	// Walk immediate children looking for identifier
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "name" {
			return child.Content(source)
		}
	}

	return ""
}

// postProcess merges small chunks and splits large ones
func (c *TreeSitterChunker) postProcess(chunks []Chunk, source []byte) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}

	var result []Chunk
	lines := strings.Split(string(source), "\n")

	for _, chunk := range chunks {
		chunkLines := chunk.EndLine - chunk.StartLine + 1

		if chunkLines > c.config.MaxChunkLines {
			// Split large chunks
			splitChunks := c.splitChunk(chunk, lines)
			result = append(result, splitChunks...)
		} else if chunkLines >= c.config.MinChunkLines {
			result = append(result, chunk)
		}
		// Skip chunks smaller than MinChunkLines (they'll be captured with context in other chunks)
	}

	return result
}

// splitChunk splits a large chunk into smaller pieces
func (c *TreeSitterChunker) splitChunk(chunk Chunk, lines []string) []Chunk {
	var result []Chunk

	totalLines := chunk.EndLine - chunk.StartLine + 1
	chunkSize := c.config.MaxChunkLines - c.config.OverlapLines

	for offset := 0; offset < totalLines; offset += chunkSize {
		startLine := chunk.StartLine + offset
		endLine := startLine + c.config.MaxChunkLines - 1
		if endLine > chunk.EndLine {
			endLine = chunk.EndLine
		}

		// Extract content
		contentLines := lines[startLine-1 : endLine]
		content := strings.Join(contentLines, "\n")

		subChunk := Chunk{
			StartLine:     startLine,
			EndLine:       endLine,
			Content:       content,
			ChunkType:     chunk.ChunkType,
			SymbolName:    chunk.SymbolName,
			Language:      chunk.Language,
			TokenEstimate: EstimateTokens(content),
		}

		result = append(result, subChunk)
	}

	return result
}

// chunkByLines is a fallback for unsupported languages
func (c *TreeSitterChunker) chunkByLines(source []byte, lang string) ([]Chunk, error) {
	lines := strings.Split(string(source), "\n")
	var chunks []Chunk

	chunkSize := c.config.MaxChunkLines
	overlap := c.config.OverlapLines

	for i := 0; i < len(lines); i += chunkSize - overlap {
		endIdx := i + chunkSize
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		if endIdx-i < c.config.MinChunkLines && len(chunks) > 0 {
			// Too small, skip
			break
		}

		content := strings.Join(lines[i:endIdx], "\n")

		chunk := Chunk{
			StartLine:     i + 1,
			EndLine:       endIdx,
			Content:       content,
			ChunkType:     "block",
			Language:      lang,
			TokenEstimate: EstimateTokens(content),
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}
