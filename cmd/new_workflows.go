package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// createGitHubWorkflows creates CI and linter workflows
func createGitHubWorkflows(targetPath, moduleName, version string) error {
	workflowsDir := filepath.Join(targetPath, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return err
	}

	// CI workflow
	ciWorkflow := fmt.Sprintf(`name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      mariadb:
        image: mariadb:10.6
        env:
          MARIADB_ROOT_PASSWORD: root
        ports:
          - 3306:3306
        options: --health-cmd="mysqladmin ping" --health-interval=5s --health-timeout=2s --health-retries=3

      redis-cache:
        image: redis:alpine
        ports:
          - 13000:6379
      redis-queue:
        image: redis:alpine
        ports:
          - 11000:6379

    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: "3.11"

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: 18

      - name: Install Frappe bench
        run: pip install frappe-bench

      - name: Initialize bench
        run: |
          bench init --skip-redis-config-generation frappe-bench
          cd frappe-bench
          bench get-app ${{ github.workspace }}

      - name: Create test site
        working-directory: frappe-bench
        run: |
          bench new-site --mariadb-root-password root --admin-password admin test.localhost
          bench --site test.localhost install-app %s

      - name: Run tests
        working-directory: frappe-bench
        run: bench --site test.localhost run-tests --app %s
`, moduleName, moduleName)

	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(ciWorkflow), 0644); err != nil {
		return err
	}

	// Linter workflow with semgrep
	linterWorkflow := `name: Linters

on:
  pull_request:
    branches: [main, develop]

jobs:
  lint:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: "3.11"

      - name: Install dependencies
        run: |
          pip install pre-commit
          pip install ruff

      - name: Run pre-commit
        run: pre-commit run --all-files

  semgrep:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Run Semgrep
        uses: returntocorp/semgrep-action@v1
        with:
          config: >-
            p/python
            p/javascript
            p/security-audit
            r/python.flask
            r/python.django

  security:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v5
        with:
          python-version: "3.11"

      - name: Install pip-audit
        run: pip install pip-audit

      - name: Run pip-audit
        run: pip-audit --strict --desc || true
`
	if err := os.WriteFile(filepath.Join(workflowsDir, "linters.yml"), []byte(linterWorkflow), 0644); err != nil {
		return err
	}

	return nil
}
