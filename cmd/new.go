package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [path]",
	Short: "Create a new Frappe app",
	Long: `Create a new Frappe app with modern project structure.

If path is "." or omitted, creates the app in the current directory.
Otherwise, creates a new directory with the app.

The command will:
1. Create the app module structure (hooks.py, __init__.py)
2. Generate pyproject.toml with [tool.weg] configuration
3. Optionally initialize the development environment

Examples:
  weg new                      # Interactive, creates in current dir
  weg new .                    # Create app in current directory
  weg new my-awesome-app       # Create new directory with app
  weg new ./apps/my-app        # Create at specific path
  weg new my-app --version 15  # Specify Frappe version`,
	Args: cobra.MaximumNArgs(1),
	RunE: runNew,
}

var (
	newAppVersion  string
	newAppDatabase string
	newAppTitle    string
	newAppAuthor   string
	newAppEmail    string
	newAppLicense  string
	newSkipInit    bool
)

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVar(&newAppVersion, "version", "", "Frappe version (14, 15, 16)")
	newCmd.Flags().StringVar(&newAppDatabase, "database", "", "Database (mariadb, postgres, sqlite)")
	newCmd.Flags().StringVar(&newAppTitle, "title", "", "App title")
	newCmd.Flags().StringVar(&newAppAuthor, "author", "", "Author name")
	newCmd.Flags().StringVar(&newAppEmail, "email", "", "Author email")
	newCmd.Flags().StringVar(&newAppLicense, "license", "MIT", "License")
	newCmd.Flags().BoolVar(&newSkipInit, "skip-init", false, "Skip initializing .weg environment")
}

func runNew(cmd *cobra.Command, args []string) error {
	// Determine path
	var targetPath string
	if len(args) == 0 || args[0] == "." {
		var err error
		targetPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
	} else {
		var err error
		targetPath, err = filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
	}

	// Determine app name from path
	appName := filepath.Base(targetPath)
	moduleName := toModuleName(appName)

	// Check if directory exists and has content
	dirExists := false
	if info, err := os.Stat(targetPath); err == nil {
		dirExists = true
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", targetPath)
		}

		// Check for existing app structure
		if _, err := os.Stat(filepath.Join(targetPath, moduleName, "hooks.py")); err == nil {
			return fmt.Errorf("app already exists at %s", targetPath)
		}
		if _, err := os.Stat(filepath.Join(targetPath, "hooks.py")); err == nil {
			return fmt.Errorf("app already exists at %s (flat structure)", targetPath)
		}
	}

	// Interactive prompts for missing info
	reader := bufio.NewReader(os.Stdin)

	// App title
	title := newAppTitle
	if title == "" {
		title = toTitle(appName)
		if !yes {
			fmt.Printf("App title [%s]: ", title)
			if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
				title = strings.TrimSpace(input)
			}
		}
	}

	// Author
	author := newAppAuthor
	if author == "" {
		author = getGitUser()
		if !yes && author == "" {
			fmt.Print("Author name: ")
			input, _ := reader.ReadString('\n')
			author = strings.TrimSpace(input)
		}
	}

	// Email
	email := newAppEmail
	if email == "" {
		email = getGitEmail()
		if !yes && email == "" {
			fmt.Print("Author email: ")
			input, _ := reader.ReadString('\n')
			email = strings.TrimSpace(input)
		}
	}

	// Frappe version
	version := newAppVersion
	if version == "" {
		version = "15"
		if !yes {
			fmt.Printf("Frappe version (14/15/16) [%s]: ", version)
			if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
				version = strings.TrimSpace(input)
			}
		}
	}

	// Validate version
	if version != "14" && version != "15" && version != "16" {
		return fmt.Errorf("invalid version: %s (must be 14, 15, or 16)", version)
	}

	// Database
	database := newAppDatabase
	if database == "" {
		if version == "16" {
			database = "sqlite"
		} else {
			database = "mariadb"
		}
		if !yes {
			opts := "mariadb/postgres"
			if version == "16" {
				opts = "mariadb/postgres/sqlite"
			}
			fmt.Printf("Database (%s) [%s]: ", opts, database)
			if input, _ := reader.ReadString('\n'); strings.TrimSpace(input) != "" {
				database = strings.TrimSpace(input)
			}
		}
	}

	PrintInfo("")
	PrintInfo("Creating Frappe app: %s", appName)
	PrintInfo("  Module: %s", moduleName)
	PrintInfo("  Path: %s", targetPath)
	PrintInfo("  Frappe: %s", version)
	PrintInfo("  Database: %s", database)
	PrintInfo("")

	// Create directory if needed
	if !dirExists {
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Create app module directory
	moduleDir := filepath.Join(targetPath, moduleName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		return fmt.Errorf("failed to create module directory: %w", err)
	}

	// Create __init__.py
	initPy := fmt.Sprintf(`__version__ = "0.0.1"
`)
	if err := os.WriteFile(filepath.Join(moduleDir, "__init__.py"), []byte(initPy), 0644); err != nil {
		return fmt.Errorf("failed to create __init__.py: %w", err)
	}

	// Create hooks.py
	hooksPy := fmt.Sprintf(`app_name = "%s"
app_title = "%s"
app_publisher = "%s"
app_description = "%s"
app_email = "%s"
app_license = "%s"

# App includes
# app_include_css = "/assets/%s/css/app.css"
# app_include_js = "/assets/%s/js/app.js"

# Website includes
# web_include_css = "/assets/%s/css/website.css"
# web_include_js = "/assets/%s/js/website.js"

# DocType Class overrides
# override_doctype_class = {
#     "ToDo": "custom_app.overrides.CustomToDo"
# }

# Document Events
# doc_events = {
#     "*": {
#         "on_update": "method",
#     }
# }

# Scheduled Tasks
# scheduler_events = {
#     "all": [
#         "%s.tasks.all"
#     ],
#     "daily": [
#         "%s.tasks.daily"
#     ],
# }
`, moduleName, title, author, title, email, newAppLicense,
		moduleName, moduleName, moduleName, moduleName, moduleName, moduleName)

	if err := os.WriteFile(filepath.Join(moduleDir, "hooks.py"), []byte(hooksPy), 0644); err != nil {
		return fmt.Errorf("failed to create hooks.py: %w", err)
	}

	// Create modules.txt
	modulesTxt := fmt.Sprintf("%s\n", title)
	if err := os.WriteFile(filepath.Join(moduleDir, "modules.txt"), []byte(modulesTxt), 0644); err != nil {
		return fmt.Errorf("failed to create modules.txt: %w", err)
	}

	// Create nested module folder (required by frappe for doctypes)
	// Structure: app_name/app_name/__init__.py
	nestedModuleDir := filepath.Join(moduleDir, moduleName)
	if err := os.MkdirAll(nestedModuleDir, 0755); err != nil {
		return fmt.Errorf("failed to create nested module directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(nestedModuleDir, "__init__.py"), []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create nested __init__.py: %w", err)
	}

	// Create pyproject.toml
	pyproject := fmt.Sprintf(`[project]
name = "%s"
version = "0.0.1"
description = "%s"
authors = [
    {name = "%s", email = "%s"}
]
license = {text = "%s"}
requires-python = ">=3.10"
readme = "README.md"

[build-system]
requires = ["flit_core >=3.2,<4"]
build-backend = "flit_core.buildapi"

[tool.weg]
# Compatibility - which Frappe versions does this app support?
[tool.weg.compatibility]
frappe = ["%s"]
databases = ["%s"]

# Development environment settings
[tool.weg.dev]
frappe = "%s"
database = "%s"

# Additional apps needed for development (optional)
# [tool.weg.dependencies]
# erpnext = { url = "https://github.com/frappe/erpnext", branch = "version-%s" }
`, appName, title, author, email, newAppLicense, version, database, version, database, version)

	if err := os.WriteFile(filepath.Join(targetPath, "pyproject.toml"), []byte(pyproject), 0644); err != nil {
		return fmt.Errorf("failed to create pyproject.toml: %w", err)
	}

	// Create README.md if it doesn't exist
	readmePath := filepath.Join(targetPath, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		codeBlock := "```"
		readme := fmt.Sprintf(`# %s

%s

## Installation

%sbash
# Using weg (recommended)
weg app get %s

# Using bench
bench get-app %s
%s

## Development

%sbash
# Clone and setup
git clone <repo-url>
cd %s
weg init
weg start
%s

## License

%s
`, title, title, codeBlock, targetPath, targetPath, codeBlock, codeBlock, appName, codeBlock, newAppLicense)

		if err := os.WriteFile(readmePath, []byte(readme), 0644); err != nil {
			PrintVerbose("Warning: failed to create README.md: %v", err)
		}
	}

	// Create .gitignore if it doesn't exist
	gitignorePath := filepath.Join(targetPath, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		gitignore := `# Byte-compiled files
__pycache__/
*.py[cod]
*$py.class
*.so

# Distribution / packaging
dist/
build/
*.egg-info/
.eggs/

# Virtual environments
.venv/
venv/
env/

# Weg development environment
.weg/

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs
*.log
logs/

# Node
node_modules/
`
		if err := os.WriteFile(gitignorePath, []byte(gitignore), 0644); err != nil {
			PrintVerbose("Warning: failed to create .gitignore: %v", err)
		}
	}

	// Create .pre-commit-config.yaml
	precommitPath := filepath.Join(targetPath, ".pre-commit-config.yaml")
	if _, err := os.Stat(precommitPath); os.IsNotExist(err) {
		precommit := fmt.Sprintf(`# See https://pre-commit.com for more information
# See https://pre-commit.com/hooks.html for more hooks
fail_fast: false

default_stages: [pre-commit, commit-msg]

repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.5.0
    hooks:
      - id: trailing-whitespace
        files: "%s.*"
      - id: check-yaml
      - id: check-json
        exclude: ".*(?:tsconfig|launch)\\.json$"
      - id: check-toml
      - id: check-ast
      - id: check-merge-conflict
      - id: debug-statements

  - repo: https://github.com/astral-sh/ruff-pre-commit
    rev: v0.7.0
    hooks:
      - id: ruff
        args: [--fix]
      - id: ruff-format

  - repo: https://github.com/pre-commit/mirrors-prettier
    rev: v4.0.0-alpha.8
    hooks:
      - id: prettier
        types_or: [javascript, vue, scss]
        exclude: |
          (?x)^(
            .*dist/.*|
            .*node_modules/.*|
            .*boilerplate/.*
          )$

  - repo: https://github.com/pre-commit/mirrors-eslint
    rev: v9.11.1
    hooks:
      - id: eslint
        args: [--quiet]
        types_or: [javascript]
        exclude: |
          (?x)^(
            .*dist/.*|
            .*node_modules/.*|
            .*boilerplate/.*
          )$

ci:
  autoupdate_schedule: weekly
  autoupdate_branch: develop
`, moduleName)
		if err := os.WriteFile(precommitPath, []byte(precommit), 0644); err != nil {
			PrintVerbose("Warning: failed to create .pre-commit-config.yaml: %v", err)
		}
	}

	// Create .editorconfig
	editorconfigPath := filepath.Join(targetPath, ".editorconfig")
	if _, err := os.Stat(editorconfigPath); os.IsNotExist(err) {
		editorconfig := `# EditorConfig helps maintain consistent coding styles
# https://editorconfig.org

root = true

[*]
charset = utf-8
end_of_line = lf
indent_style = tab
insert_final_newline = true
trim_trailing_whitespace = true

[*.{py,rst}]
indent_style = space
indent_size = 4

[*.{js,jsx,ts,tsx,vue,css,scss,json,yml,yaml,html}]
indent_style = tab
indent_size = 4

[*.md]
trim_trailing_whitespace = false
`
		if err := os.WriteFile(editorconfigPath, []byte(editorconfig), 0644); err != nil {
			PrintVerbose("Warning: failed to create .editorconfig: %v", err)
		}
	}

	// Create .eslintrc
	eslintrcPath := filepath.Join(targetPath, ".eslintrc")
	if _, err := os.Stat(eslintrcPath); os.IsNotExist(err) {
		eslintrc := `{
	"env": {
		"browser": true,
		"node": true,
		"es2022": true
	},
	"parserOptions": {
		"ecmaVersion": "latest",
		"sourceType": "module"
	},
	"extends": "eslint:recommended",
	"rules": {
		"indent": ["error", "tab"],
		"linebreak-style": ["error", "unix"],
		"no-console": "warn",
		"no-unused-vars": ["error", { "argsIgnorePattern": "^_" }]
	},
	"globals": {
		"frappe": "readonly",
		"$": "readonly",
		"__": "readonly",
		"locals": "readonly",
		"cur_frm": "readonly",
		"cur_dialog": "readonly",
		"cur_page": "readonly",
		"cur_list": "readonly"
	}
}
`
		if err := os.WriteFile(eslintrcPath, []byte(eslintrc), 0644); err != nil {
			PrintVerbose("Warning: failed to create .eslintrc: %v", err)
		}
	}

	// Create GitHub Actions workflows
	if err := createGitHubWorkflows(targetPath, moduleName, version); err != nil {
		PrintVerbose("Warning: failed to create GitHub workflows: %v", err)
	}

	PrintInfo("App structure created")

	// Initialize weg environment
	if !newSkipInit {
		PrintInfo("")
		PrintInfo("Initializing development environment...")

		// Create .weg directory structure
		wegPath := filepath.Join(targetPath, ".weg")
		if err := os.MkdirAll(filepath.Join(wegPath, "apps"), 0755); err != nil {
			return fmt.Errorf("failed to create .weg/apps: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(wegPath, "sites"), 0755); err != nil {
			return fmt.Errorf("failed to create .weg/sites: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(wegPath, "config", "pids"), 0755); err != nil {
			return fmt.Errorf("failed to create .weg/config/pids: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(wegPath, "logs"), 0755); err != nil {
			return fmt.Errorf("failed to create .weg/logs: %w", err)
		}

		// Create weg.toml in .weg
		siteName := fmt.Sprintf("%s.localhost", moduleName)
		wegToml := fmt.Sprintf(`# Generated by weg new
[bench]
name = "%s-dev"

[frappe]
version = "%s"
database = "%s"

[apps.frappe]
url = "https://github.com/frappe/frappe"
branch = "version-%s"

[apps.%s]
path = ".."

[[sites]]
name = "%s"
default = true
apps = ["frappe", "%s"]
`, appName, version, database, version, moduleName, siteName, moduleName)

		if err := os.WriteFile(filepath.Join(wegPath, "weg.toml"), []byte(wegToml), 0644); err != nil {
			return fmt.Errorf("failed to create .weg/weg.toml: %w", err)
		}

		// Initialize devbox environment
		if err := initDevboxEnvironment(wegPath, version); err != nil {
			return fmt.Errorf("failed to initialize devbox environment: %w", err)
		}

		PrintInfo("Development environment initialized")
	}

	PrintInfo("")
	PrintInfo("App created successfully!")
	PrintInfo("")
	PrintInfo("Next steps:")
	if targetPath != "." {
		PrintInfo("  cd %s", targetPath)
	}
	PrintInfo("  weg sync        # Install Frappe and dependencies")
	PrintInfo("  weg start       # Start development server")

	return nil
}
