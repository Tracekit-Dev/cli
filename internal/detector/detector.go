package detector

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Framework represents a detected framework
type Framework struct {
	Name    string // "gemvc", "laravel", "express", "django", etc.
	Version string // Framework version (if detectable)
	Type    string // "go", "php", "node", "python", etc.
}

// Detect attempts to detect the framework in the current directory
func Detect() (*Framework, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Check for Go frameworks
	if fileExists(filepath.Join(cwd, "go.mod")) {
		return detectGoFramework(cwd)
	}

	// Check for PHP frameworks
	if fileExists(filepath.Join(cwd, "composer.json")) {
		return detectPHPFramework(cwd)
	}

	// Check for Node.js frameworks
	if fileExists(filepath.Join(cwd, "package.json")) {
		return detectNodeFramework(cwd)
	}

	// Check for Python frameworks
	if fileExists(filepath.Join(cwd, "requirements.txt")) || fileExists(filepath.Join(cwd, "pyproject.toml")) {
		return detectPythonFramework(cwd)
	}

	// Check for Ruby frameworks
	if fileExists(filepath.Join(cwd, "Gemfile")) {
		return detectRubyFramework(cwd)
	}

	// No framework detected - return generic
	return &Framework{
		Name: "generic",
		Type: "unknown",
	}, nil
}

func detectGoFramework(dir string) (*Framework, error) {
	goModPath := filepath.Join(dir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}

	goModContent := string(content)

	// Check for Gin
	if strings.Contains(goModContent, "github.com/gin-gonic/gin") {
		return &Framework{
			Name: "gin",
			Type: "go",
		}, nil
	}

	// Check for Echo
	if strings.Contains(goModContent, "github.com/labstack/echo") {
		return &Framework{
			Name: "echo",
			Type: "go",
		}, nil
	}

	// Check for Fiber
	if strings.Contains(goModContent, "github.com/gofiber/fiber") {
		return &Framework{
			Name: "fiber",
			Type: "go",
		}, nil
	}

	// Generic Go project
	return &Framework{
		Name: "go",
		Type: "go",
	}, nil
}

func detectPHPFramework(dir string) (*Framework, error) {
	composerPath := filepath.Join(dir, "composer.json")
	content, err := os.ReadFile(composerPath)
	if err != nil {
		return nil, err
	}

	composerContent := string(content)

	// Check for GemVC (PHP framework)
	if strings.Contains(composerContent, "gemvc/library") {
		return &Framework{
			Name: "gemvc",
			Type: "php",
		}, nil
	}

	// Check for Laravel
	if strings.Contains(composerContent, "laravel/framework") {
		return &Framework{
			Name: "laravel",
			Type: "php",
		}, nil
	}

	// Check for Symfony
	if strings.Contains(composerContent, "symfony/symfony") {
		return &Framework{
			Name: "symfony",
			Type: "php",
		}, nil
	}

	// Generic PHP project
	return &Framework{
		Name: "php",
		Type: "php",
	}, nil
}

func detectNodeFramework(dir string) (*Framework, error) {
	packagePath := filepath.Join(dir, "package.json")
	content, err := os.ReadFile(packagePath)
	if err != nil {
		return nil, err
	}

	packageContent := string(content)

	// Check for Express
	if strings.Contains(packageContent, "\"express\"") {
		return &Framework{
			Name: "express",
			Type: "node",
		}, nil
	}

	// Check for Next.js
	if strings.Contains(packageContent, "\"next\"") {
		return &Framework{
			Name: "nextjs",
			Type: "node",
		}, nil
	}

	// Check for NestJS
	if strings.Contains(packageContent, "@nestjs/core") {
		return &Framework{
			Name: "nestjs",
			Type: "node",
		}, nil
	}

	// Generic Node.js project
	return &Framework{
		Name: "node",
		Type: "node",
	}, nil
}

func detectPythonFramework(dir string) (*Framework, error) {
	// Check requirements.txt
	reqPath := filepath.Join(dir, "requirements.txt")
	if fileExists(reqPath) {
		content, err := os.ReadFile(reqPath)
		if err == nil {
			reqContent := string(content)

			if strings.Contains(reqContent, "Django") {
				return &Framework{
					Name: "django",
					Type: "python",
				}, nil
			}

			if strings.Contains(reqContent, "Flask") {
				return &Framework{
					Name: "flask",
					Type: "python",
				}, nil
			}

			if strings.Contains(reqContent, "fastapi") {
				return &Framework{
					Name: "fastapi",
					Type: "python",
				}, nil
			}
		}
	}

	// Generic Python project
	return &Framework{
		Name: "python",
		Type: "python",
	}, nil
}

func detectRubyFramework(dir string) (*Framework, error) {
	gemfilePath := filepath.Join(dir, "Gemfile")
	content, err := os.ReadFile(gemfilePath)
	if err != nil {
		return nil, err
	}

	gemfileContent := string(content)

	// Check for Rails
	if strings.Contains(gemfileContent, "rails") {
		return &Framework{
			Name: "rails",
			Type: "ruby",
		}, nil
	}

	// Check for Sinatra
	if strings.Contains(gemfileContent, "sinatra") {
		return &Framework{
			Name: "sinatra",
			Type: "ruby",
		}, nil
	}

	// Generic Ruby project
	return &Framework{
		Name: "ruby",
		Type: "ruby",
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// healthPatterns are the endpoint paths we scan for in source files.
var healthPatterns = []string{"/health", "/healthz", "/ready", "/readiness", "/liveness"}

// skipDirs are directories we never descend into during health endpoint scanning.
var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
}

// DetectHealthEndpoints scans the current directory for health check endpoint
// registrations based on the detected framework. It walks files up to 3 levels
// deep, reading at most 64KB per file, and returns a deduplicated list of
// detected endpoint paths.
func DetectHealthEndpoints(framework *Framework) []string {
	cwd, err := os.Getwd()
	if err != nil {
		return []string{}
	}

	// Determine which file extensions to scan based on framework type
	exts := extensionsForFramework(framework)

	seen := make(map[string]bool)
	baseDepth := strings.Count(filepath.ToSlash(cwd), "/")

	_ = filepath.WalkDir(cwd, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		// Enforce max depth of 3
		depth := strings.Count(filepath.ToSlash(path), "/") - baseDepth
		if depth > 3 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored directories
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Filter by extension
		ext := strings.ToLower(filepath.Ext(path))
		if !exts[ext] {
			return nil
		}

		// Read file content (max 64KB)
		content := readFileHead(path, 64*1024)
		if content == "" {
			return nil
		}

		// Extract health endpoints from content
		for _, ep := range extractHealthEndpoints(content) {
			seen[ep] = true
		}

		return nil
	})

	// Deduplicated results
	endpoints := make([]string, 0, len(seen))
	for ep := range seen {
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

// extensionsForFramework returns the set of file extensions to scan.
func extensionsForFramework(fw *Framework) map[string]bool {
	if fw == nil {
		return genericExtensions()
	}
	switch fw.Type {
	case "go":
		return map[string]bool{".go": true}
	case "node":
		return map[string]bool{".js": true, ".ts": true, ".mjs": true}
	case "python":
		return map[string]bool{".py": true}
	case "php":
		return map[string]bool{".php": true}
	case "ruby":
		return map[string]bool{".rb": true}
	default:
		return genericExtensions()
	}
}

func genericExtensions() map[string]bool {
	return map[string]bool{
		".go": true, ".js": true, ".ts": true, ".mjs": true,
		".py": true, ".php": true, ".rb": true,
	}
}

// readFileHead reads up to maxBytes from a file. Returns empty string on error.
func readFileHead(path string, maxBytes int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	lr := io.LimitReader(f, maxBytes)
	data, err := io.ReadAll(lr)
	if err != nil {
		return ""
	}
	return string(data)
}

// extractHealthEndpoints finds health-related endpoint paths in file content.
// It looks for common route registration patterns and plain string matches.
func extractHealthEndpoints(content string) []string {
	var found []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, pattern := range healthPatterns {
			if !strings.Contains(lower, pattern) {
				continue
			}
			// Try to extract the actual path from the line
			ep := extractPath(line, pattern)
			if ep != "" {
				found = append(found, ep)
			}
		}
	}
	return found
}

// extractPath attempts to extract a URL path from a source code line.
// It looks for quoted strings containing the health pattern.
func extractPath(line string, pattern string) string {
	// Look for the pattern inside quotes (single or double)
	for _, quote := range []byte{'"', '\'', '`'} {
		idx := 0
		for {
			start := strings.IndexByte(line[idx:], quote)
			if start == -1 {
				break
			}
			start += idx
			end := strings.IndexByte(line[start+1:], quote)
			if end == -1 {
				break
			}
			end += start + 1
			quoted := line[start+1 : end]
			if strings.Contains(strings.ToLower(quoted), pattern) && strings.HasPrefix(quoted, "/") {
				// Clean: take just the path portion (stop at whitespace, quote, or comma)
				return cleanPath(quoted)
			}
			idx = end + 1
		}
	}
	return ""
}

// cleanPath extracts a clean URL path, stopping at query strings or whitespace.
func cleanPath(s string) string {
	// Trim trailing characters that are not path-like
	s = strings.TrimRight(s, " \t\r\n,;")
	// Stop at query string
	if idx := strings.IndexByte(s, '?'); idx != -1 {
		s = s[:idx]
	}
	// Ensure it starts with /
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	return s
}
