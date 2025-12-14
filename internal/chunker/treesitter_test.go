package chunker

import (
	"strings"
	"testing"
)

func TestTreeSitterChunkerSupportedLanguages(t *testing.T) {
	chunker := NewTreeSitter(DefaultConfig())
	languages := chunker.SupportedLanguages()

	expected := []string{"go", "python", "javascript", "typescript", "rust", "ruby", "java", "c", "cpp", "csharp", "bash"}
	if len(languages) != len(expected) {
		t.Errorf("SupportedLanguages() returned %d languages, want %d", len(languages), len(expected))
	}

	for _, lang := range expected {
		found := false
		for _, l := range languages {
			if l == lang {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected language %q not found", lang)
		}
	}
}

func TestTreeSitterChunkGo(t *testing.T) {
	source := `package main

import "fmt"

func hello() {
	fmt.Println("Hello")
}

func world() {
	fmt.Println("World")
}

type User struct {
	Name string
	Age  int
}
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 2,
		OverlapLines:  1,
	})

	chunks, err := chunker.Chunk([]byte(source), "main.go")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Check that we got function chunks
	var functionCount int
	for _, c := range chunks {
		if c.ChunkType == "function" {
			functionCount++
		}
		if c.Language != "go" {
			t.Errorf("chunk language = %q, want go", c.Language)
		}
	}
	if functionCount < 2 {
		t.Errorf("expected at least 2 function chunks, got %d", functionCount)
	}
}

func TestTreeSitterChunkPython(t *testing.T) {
	source := `import os

def greet(name):
    print(f"Hello, {name}")

class User:
    def __init__(self, name):
        self.name = name

    def say_hello(self):
        print(f"Hi, I'm {self.name}")
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 2,
		OverlapLines:  1,
	})

	chunks, err := chunker.Chunk([]byte(source), "app.py")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Check for function and class chunks
	var hasFunction, hasClass bool
	for _, c := range chunks {
		if c.ChunkType == "function" {
			hasFunction = true
		}
		if c.ChunkType == "class" {
			hasClass = true
		}
	}
	if !hasFunction {
		t.Error("expected at least one function chunk")
	}
	if !hasClass {
		t.Error("expected at least one class chunk")
	}
}

func TestTreeSitterChunkTypeScript(t *testing.T) {
	source := `interface User {
  name: string;
  age: number;
}

function createUser(name: string): User {
  return { name, age: 0 };
}

class UserService {
  private users: User[] = [];

  addUser(user: User): void {
    this.users.push(user);
  }
}
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 2,
		OverlapLines:  1,
	})

	chunks, err := chunker.Chunk([]byte(source), "app.ts")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	for _, c := range chunks {
		if c.Language != "typescript" {
			t.Errorf("chunk language = %q, want typescript", c.Language)
		}
	}
}

func TestTreeSitterChunkRust(t *testing.T) {
	source := `struct Point {
    x: f64,
    y: f64,
}

impl Point {
    fn new(x: f64, y: f64) -> Self {
        Point { x, y }
    }

    fn distance(&self, other: &Point) -> f64 {
        ((self.x - other.x).powi(2) + (self.y - other.y).powi(2)).sqrt()
    }
}

fn main() {
    let p = Point::new(0.0, 0.0);
    println!("{}", p.x);
}
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 2,
		OverlapLines:  1,
	})

	chunks, err := chunker.Chunk([]byte(source), "main.rs")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Should have struct, impl, and function chunks
	var hasStruct, hasImpl, hasFunction bool
	for _, c := range chunks {
		switch c.ChunkType {
		case "struct":
			hasStruct = true
		case "impl":
			hasImpl = true
		case "function":
			hasFunction = true
		}
	}
	if !hasStruct {
		t.Error("expected struct chunk")
	}
	if !hasImpl {
		t.Error("expected impl chunk")
	}
	if !hasFunction {
		t.Error("expected function chunk")
	}
}

func TestTreeSitterChunkLargeFunction(t *testing.T) {
	// Create a large function that should be split
	var lines []string
	lines = append(lines, "func bigFunction() {")
	for i := 0; i < 150; i++ {
		lines = append(lines, "    // line "+string(rune('A'+i%26)))
	}
	lines = append(lines, "}")
	source := strings.Join(lines, "\n")

	chunker := NewTreeSitter(Config{
		MaxChunkLines: 50,
		MinChunkLines: 5,
		OverlapLines:  5,
	})

	chunks, err := chunker.Chunk([]byte(source), "big.go")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Should be split into multiple chunks
	if len(chunks) < 2 {
		t.Errorf("expected large function to be split, got %d chunks", len(chunks))
	}

	// Each chunk should be at most MaxChunkLines
	for i, c := range chunks {
		lines := c.EndLine - c.StartLine + 1
		if lines > 50 {
			t.Errorf("chunk %d has %d lines, want <= 50", i, lines)
		}
	}
}

func TestTreeSitterChunkByLinesFallback(t *testing.T) {
	// Test fallback for unsupported language patterns
	source := `# Some unsupported format
data = [1, 2, 3]
more_data = [4, 5, 6]
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 1,
		OverlapLines:  1,
	})

	// This should fall back to line-based chunking
	chunks, err := chunker.chunkByLines([]byte(source), "unknown")
	if err != nil {
		t.Fatalf("chunkByLines failed: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("expected at least 1 chunk from fallback")
	}
}

func TestTreeSitterSymbolExtraction(t *testing.T) {
	source := `package main

func calculateSum(a, b int) int {
	return a + b
}

type Calculator struct {
	value int
}
`
	chunker := NewTreeSitter(Config{
		MaxChunkLines: 100,
		MinChunkLines: 2,
		OverlapLines:  1,
	})

	chunks, err := chunker.Chunk([]byte(source), "calc.go")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Look for the function with its name
	var foundFunction bool
	for _, c := range chunks {
		if c.ChunkType == "function" && c.SymbolName == "calculateSum" {
			foundFunction = true
			break
		}
	}
	if !foundFunction {
		t.Error("expected to find function 'calculateSum' with extracted symbol name")
	}
}

func TestTreeSitterEmptyFile(t *testing.T) {
	chunker := NewTreeSitter(DefaultConfig())

	chunks, err := chunker.Chunk([]byte(""), "empty.go")
	if err != nil {
		t.Fatalf("Chunk failed: %v", err)
	}

	// Empty file should produce no chunks (or fall back gracefully)
	// This is acceptable behavior
	_ = chunks
}

func TestTreeSitterUnsupportedFile(t *testing.T) {
	chunker := NewTreeSitter(DefaultConfig())

	_, err := chunker.Chunk([]byte("some content"), "file.unknown")
	if err == nil {
		t.Error("expected error for unsupported file type")
	}
}
