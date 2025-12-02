package sdk

import (
	"fmt"
	"os/exec"
	"runtime"
)

// SDK represents an SDK installation option
type SDK struct {
	Name        string
	Language    string
	PackageName string
	InstallCmd  string
	Description string
}

// GetAvailableSDKs returns list of all available SDKs
func GetAvailableSDKs() []SDK {
	return []SDK{
		{
			Name:        "PHP",
			Language:    "php",
			PackageName: "tracekit/php-apm",
			InstallCmd:  "composer require tracekit/php-apm",
			Description: "TraceKit PHP SDK (works with any PHP project)",
		},
		{
			Name:        "Laravel",
			Language:    "php",
			PackageName: "tracekit/laravel-apm",
			InstallCmd:  "composer require tracekit/laravel-apm",
			Description: "TraceKit Laravel SDK (optimized for Laravel)",
		},
		{
			Name:        "Node.js",
			Language:    "node",
			PackageName: "@tracekit/node-apm",
			InstallCmd:  "npm install @tracekit/node-apm",
			Description: "TraceKit Node.js SDK (Express, Fastify, etc.)",
		},
		{
			Name:        "Go",
			Language:    "go",
			PackageName: "github.com/Tracekit-Dev/go-sdk",
			InstallCmd:  "go get github.com/Tracekit-Dev/go-sdk",
			Description: "TraceKit Go SDK (Gin, Echo, Fiber, net/http)",
		},
		{
			Name:        "Python",
			Language:    "python",
			PackageName: "tracekit-python",
			InstallCmd:  "pip install tracekit-python",
			Description: "TraceKit Python SDK (Django, Flask, FastAPI)",
		},
	}
}

// GetRecommendedSDK returns the recommended SDK based on framework type
func GetRecommendedSDK(frameworkType, frameworkName string) *SDK {
	sdks := GetAvailableSDKs()

	// Special case for Laravel
	if frameworkName == "laravel" {
		for _, sdk := range sdks {
			if sdk.Name == "Laravel" {
				return &sdk
			}
		}
	}

	// Match by language type
	for _, sdk := range sdks {
		if sdk.Language == frameworkType {
			return &sdk
		}
	}

	return nil
}

// Install runs the SDK installation command
func Install(sdk SDK) error {
	var cmd *exec.Cmd

	switch sdk.Language {
	case "php":
		// Check if composer exists
		if !commandExists("composer") {
			return fmt.Errorf("composer not found - please install composer first: https://getcomposer.org")
		}
		cmd = exec.Command("composer", "require", sdk.PackageName)

	case "node":
		// Check if npm exists, fallback to yarn
		if commandExists("npm") {
			cmd = exec.Command("npm", "install", sdk.PackageName)
		} else if commandExists("yarn") {
			cmd = exec.Command("yarn", "add", sdk.PackageName)
		} else {
			return fmt.Errorf("npm or yarn not found - please install Node.js first: https://nodejs.org")
		}

	case "go":
		// Check if go exists
		if !commandExists("go") {
			return fmt.Errorf("go not found - please install Go first: https://go.dev")
		}
		cmd = exec.Command("go", "get", sdk.PackageName)

	case "python":
		// Check if pip exists, fallback to pip3
		if commandExists("pip") {
			cmd = exec.Command("pip", "install", sdk.PackageName)
		} else if commandExists("pip3") {
			cmd = exec.Command("pip3", "install", sdk.PackageName)
		} else {
			return fmt.Errorf("pip not found - please install Python first: https://python.org")
		}

	default:
		return fmt.Errorf("unsupported SDK language: %s", sdk.Language)
	}

	// Set environment and run
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run()
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// GetInstallInstructions returns manual installation instructions for an SDK
func GetInstallInstructions(sdk SDK) []string {
	instructions := []string{
		fmt.Sprintf("Install %s SDK:", sdk.Name),
		"  " + sdk.InstallCmd,
		"",
	}

	// Add initialization example based on language
	switch sdk.Language {
	case "php":
		if sdk.Name == "Laravel" {
			instructions = append(instructions,
				"Laravel auto-discovery will register the service provider.",
				"Publish config (optional): php artisan vendor:publish --tag=tracekit",
			)
		} else {
			instructions = append(instructions,
				"Require in your code:",
				"  require 'vendor/autoload.php';",
				"  TraceKit\\SDK::init();",
			)
		}

	case "node":
		instructions = append(instructions,
			"Import in your code:",
			"  const tracekit = require('@tracekit/node-apm');",
			"  tracekit.init();",
		)

	case "go":
		instructions = append(instructions,
			"Import in your code:",
			"  import \"github.com/Tracekit-Dev/go-sdk\"",
			"  tracekit.Init()",
		)

	case "python":
		instructions = append(instructions,
			"Import in your code:",
			"  import tracekit",
			"  tracekit.init()",
		)
	}

	return instructions
}

// IsWindows checks if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
