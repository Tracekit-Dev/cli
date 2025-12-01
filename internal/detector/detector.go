package detector

import (
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
