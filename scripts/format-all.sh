#!/usr/bin/env bash
# format-all.sh - Autoformat entire MantisDB codebase to style guide

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔧 Formatting MantisDB codebase..."
echo "Project root: $PROJECT_ROOT"
cd "$PROJECT_ROOT"

# Go formatting
echo ""
echo "📦 Formatting Go code..."
if command -v gofmt &> /dev/null; then
	# Format all Go files with tabs (gofmt default)
	find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" -not -path "*/node_modules/*" | while read -r file; do
		gofmt -w "$file"
	done
	echo "✓ Go code formatted"
else
	echo "⚠️  gofmt not found, skipping Go formatting"
fi

# Rust formatting
echo ""
echo "🦀 Formatting Rust code..."
if [ -d "rust-core" ]; then
	cd rust-core
	if command -v cargo &> /dev/null; then
		cargo fmt --all
		echo "✓ Rust code formatted"
	else
		echo "⚠️  cargo not found, skipping Rust formatting"
	fi
	cd "$PROJECT_ROOT"
fi

# TypeScript/JavaScript formatting (admin frontend)
echo ""
echo "📜 Formatting TypeScript/JavaScript..."
if [ -d "admin/frontend" ]; then
	cd admin/frontend
	if [ -f "package.json" ] && command -v npm &> /dev/null; then
		if npm list eslint &> /dev/null || grep -q "eslint" package.json; then
			echo "Running ESLint fix..."
			npm run lint --fix 2>/dev/null || npx eslint --fix src/ 2>/dev/null || echo "⚠️  ESLint not configured"
		fi
		echo "✓ TypeScript/JavaScript formatted"
	else
		echo "⚠️  npm not available, skipping TS/JS formatting"
	fi
	cd "$PROJECT_ROOT"
fi

# TypeScript SDK
if [ -d "sdks/typescript" ]; then
	cd sdks/typescript
	if [ -f "package.json" ] && command -v npm &> /dev/null; then
		if npm list eslint &> /dev/null || grep -q "eslint" package.json; then
			npm run lint --fix 2>/dev/null || npx eslint --fix src/ 2>/dev/null || echo "⚠️  ESLint not configured"
		fi
	fi
	cd "$PROJECT_ROOT"
fi

# Python formatting
echo ""
echo "🐍 Formatting Python code..."
if [ -d "sdks/python" ]; then
	cd sdks/python
	if command -v ruff &> /dev/null; then
		ruff format .
		echo "✓ Python code formatted with ruff"
	elif command -v black &> /dev/null; then
		black .
		echo "✓ Python code formatted with black"
	else
		echo "⚠️  ruff/black not found, skipping Python formatting"
	fi
	cd "$PROJECT_ROOT"
fi

echo ""
echo "✅ Formatting complete!"
echo ""
echo "Next steps:"
echo "  1. Review changes: git diff"
echo "  2. Run tests: make test"
echo "  3. Commit: git commit -am 'style: apply CODE_STYLE.md formatting'"
