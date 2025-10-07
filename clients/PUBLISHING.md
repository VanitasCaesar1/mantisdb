# MantisDB Client SDK Publishing Guide

Complete guide for publishing MantisDB client libraries to package registries.

## Table of Contents

- [JavaScript/TypeScript (npm)](#javascripttypescript-npm)
- [Python (PyPI)](#python-pypi)
- [Go (pkg.go.dev)](#go-pkggodev)
- [Version Management](#version-management)
- [Release Checklist](#release-checklist)

---

## JavaScript/TypeScript (npm)

### Package Details

- **Package Name**: `@mantisdb/client`
- **Registry**: npm (https://www.npmjs.com)
- **Location**: `clients/javascript/`

### Prerequisites

1. **npm account**: Create at https://www.npmjs.com/signup
2. **npm login**: `npm login`
3. **Organization** (optional): Create `@mantisdb` organization on npm

### Publishing Steps

#### 1. Prepare the Package

```bash
cd clients/javascript

# Install dependencies
npm install

# Build the package
npm run build

# Run tests
npm test

# Lint code
npm run lint
```

#### 2. Update Version

```bash
# Update version in package.json
npm version patch  # 1.0.0 -> 1.0.1
npm version minor  # 1.0.0 -> 1.1.0
npm version major  # 1.0.0 -> 2.0.0

# Or manually edit package.json
```

#### 3. Test Package Locally

```bash
# Create tarball
npm pack

# Test installation
npm install mantisdb-client-1.0.0.tgz
```

#### 4. Publish to npm

```bash
# Dry run (see what will be published)
npm publish --dry-run

# Publish to npm
npm publish --access public

# For scoped packages (@mantisdb/client)
npm publish --access public
```

#### 5. Verify Publication

```bash
# Check on npm
open https://www.npmjs.com/package/@mantisdb/client

# Test installation
npm install @mantisdb/client
```

### Automated Publishing (GitHub Actions)

Create `.github/workflows/publish-npm.yml`:

```yaml
name: Publish to npm

on:
  release:
    types: [created]

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
          registry-url: 'https://registry.npmjs.org'
      
      - name: Install dependencies
        working-directory: clients/javascript
        run: npm ci
      
      - name: Build
        working-directory: clients/javascript
        run: npm run build
      
      - name: Test
        working-directory: clients/javascript
        run: npm test
      
      - name: Publish
        working-directory: clients/javascript
        run: npm publish --access public
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
```

### npm Token Setup

1. Generate token: https://www.npmjs.com/settings/YOUR_USERNAME/tokens
2. Add to GitHub Secrets: `NPM_TOKEN`

---

## Python (PyPI)

### Package Details

- **Package Name**: `mantisdb-python` or `mantisdb`
- **Registry**: PyPI (https://pypi.org)
- **Location**: `clients/python/`

### Prerequisites

1. **PyPI account**: Create at https://pypi.org/account/register/
2. **TestPyPI account** (optional): https://test.pypi.org/account/register/
3. **Build tools**: `pip install build twine`

### Publishing Steps

#### 1. Prepare the Package

```bash
cd clients/python

# Install in development mode
pip install -e .[dev]

# Run tests
pytest

# Type checking
mypy .

# Format code
black .
isort .
```

#### 2. Update Version

Edit `pyproject.toml`:

```toml
[project]
name = "mantisdb"
version = "1.0.0"  # Update this
```

#### 3. Build the Package

```bash
# Clean previous builds
rm -rf dist/ build/ *.egg-info

# Build source distribution and wheel
python -m build

# This creates:
# - dist/mantisdb-1.0.0.tar.gz (source distribution)
# - dist/mantisdb-1.0.0-py3-none-any.whl (wheel)
```

#### 4. Test on TestPyPI (Optional but Recommended)

```bash
# Upload to TestPyPI
python -m twine upload --repository testpypi dist/*

# Test installation
pip install --index-url https://test.pypi.org/simple/ mantisdb
```

#### 5. Publish to PyPI

```bash
# Check package
python -m twine check dist/*

# Upload to PyPI
python -m twine upload dist/*

# You'll be prompted for username and password
# Or use API token (recommended)
```

#### 6. Verify Publication

```bash
# Check on PyPI
open https://pypi.org/project/mantisdb/

# Test installation
pip install mantisdb
```

### Using API Tokens (Recommended)

1. Generate token: https://pypi.org/manage/account/token/
2. Create `~/.pypirc`:

```ini
[pypi]
username = __token__
password = pypi-YOUR_TOKEN_HERE

[testpypi]
username = __token__
password = pypi-YOUR_TOKEN_HERE
```

Or use environment variable:

```bash
export TWINE_USERNAME=__token__
export TWINE_PASSWORD=pypi-YOUR_TOKEN_HERE
python -m twine upload dist/*
```

### Automated Publishing (GitHub Actions)

Create `.github/workflows/publish-pypi.yml`:

```yaml
name: Publish to PyPI

on:
  release:
    types: [created]

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-python@v4
        with:
          python-version: '3.11'
      
      - name: Install dependencies
        run: |
          python -m pip install --upgrade pip
          pip install build twine
      
      - name: Build package
        working-directory: clients/python
        run: python -m build
      
      - name: Publish to PyPI
        working-directory: clients/python
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_TOKEN }}
        run: python -m twine upload dist/*
```

### PyPI Token Setup

1. Generate token: https://pypi.org/manage/account/token/
2. Add to GitHub Secrets: `PYPI_TOKEN`

---

## Go (pkg.go.dev)

### Package Details

- **Package Name**: `github.com/mantisdb/mantisdb/clients/go`
- **Registry**: pkg.go.dev (automatic)
- **Location**: `clients/go/`

### Prerequisites

1. **GitHub repository**: Must be public
2. **Go modules**: Already using `go.mod`
3. **Git tags**: For versioning

### Publishing Steps

#### 1. Prepare the Package

```bash
cd clients/go

# Format code
go fmt ./...

# Vet code
go vet ./...

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

#### 2. Create Git Tag

Go uses **semantic versioning** with git tags:

```bash
# Tag the entire repository
cd /path/to/mantisdb

# Create tag for Go client
git tag clients/go/v1.0.0
git push origin clients/go/v1.0.0

# Or for major versions >= 2, update go.mod:
# module github.com/mantisdb/mantisdb/clients/go/v2
```

#### 3. Automatic Publication

**pkg.go.dev automatically indexes your package** when:
- Repository is public on GitHub
- You push a git tag
- Someone runs `go get github.com/mantisdb/mantisdb/clients/go@v1.0.0`

#### 4. Trigger Indexing

```bash
# Request indexing
curl https://proxy.golang.org/github.com/mantisdb/mantisdb/clients/go/@v/v1.0.0.info

# Or have someone install it
go get github.com/mantisdb/mantisdb/clients/go@v1.0.0
```

#### 5. Verify Publication

```bash
# Check on pkg.go.dev
open https://pkg.go.dev/github.com/mantisdb/mantisdb/clients/go

# Test installation
go get github.com/mantisdb/mantisdb/clients/go@v1.0.0
```

### Go Module Best Practices

#### Version Tags

```bash
# Patch release (bug fixes)
git tag clients/go/v1.0.1

# Minor release (new features, backward compatible)
git tag clients/go/v1.1.0

# Major release (breaking changes)
git tag clients/go/v2.0.0
# Also update go.mod: module github.com/mantisdb/mantisdb/clients/go/v2
```

#### Documentation

Add documentation comments:

```go
// Package mantisdb provides a Go client for MantisDB.
//
// Example usage:
//
//	client := mantisdb.NewClient("http://localhost:8080")
//	err := client.Set("key", "value")
package mantisdb
```

### Automated Publishing (GitHub Actions)

Create `.github/workflows/publish-go.yml`:

```yaml
name: Publish Go Module

on:
  push:
    tags:
      - 'clients/go/v*'

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Test
        working-directory: clients/go
        run: |
          go test ./...
          go vet ./...
      
      - name: Trigger pkg.go.dev indexing
        run: |
          VERSION=${GITHUB_REF#refs/tags/clients/go/}
          curl "https://proxy.golang.org/github.com/mantisdb/mantisdb/clients/go/@v/${VERSION}.info"
```

---

## Version Management

### Semantic Versioning

All clients follow [Semantic Versioning](https://semver.org/):

- **MAJOR** (1.0.0 → 2.0.0): Breaking changes
- **MINOR** (1.0.0 → 1.1.0): New features, backward compatible
- **PATCH** (1.0.0 → 1.0.1): Bug fixes

### Synchronized Versions

Keep all client versions synchronized with MantisDB releases:

```bash
# Update all clients to version 1.2.0
./scripts/update-client-versions.sh 1.2.0
```

Create `scripts/update-client-versions.sh`:

```bash
#!/bin/bash
VERSION=$1

# Update JavaScript
sed -i '' "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" clients/javascript/package.json

# Update Python
sed -i '' "s/version = \".*\"/version = \"$VERSION\"/" clients/python/pyproject.toml

# Update Go (create tag)
git tag "clients/go/v$VERSION"

echo "Updated all clients to version $VERSION"
```

---

## Release Checklist

### Pre-Release

- [ ] Update CHANGELOG.md for each client
- [ ] Update version numbers
- [ ] Run all tests
- [ ] Update documentation
- [ ] Review breaking changes
- [ ] Test examples still work

### JavaScript/TypeScript

- [ ] `npm run build` succeeds
- [ ] `npm test` passes
- [ ] `npm run lint` passes
- [ ] README.md is up to date
- [ ] Examples are tested

### Python

- [ ] `pytest` passes
- [ ] `mypy .` passes
- [ ] `black .` and `isort .` run
- [ ] README.md is up to date
- [ ] Examples are tested

### Go

- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go fmt ./...` run
- [ ] Documentation comments updated
- [ ] Examples are tested

### Publishing

- [ ] Publish to npm: `npm publish`
- [ ] Publish to PyPI: `twine upload dist/*`
- [ ] Tag Go release: `git tag clients/go/vX.Y.Z`
- [ ] Create GitHub release
- [ ] Update documentation website
- [ ] Announce on social media/blog

### Post-Release

- [ ] Verify packages are live
- [ ] Test installation from registries
- [ ] Update main README.md
- [ ] Close milestone on GitHub
- [ ] Thank contributors

---

## Quick Commands

### Publish All Clients

```bash
# JavaScript
cd clients/javascript && npm publish --access public

# Python
cd clients/python && python -m twine upload dist/*

# Go
git tag clients/go/v1.0.0 && git push origin clients/go/v1.0.0
```

### Test All Clients

```bash
# JavaScript
cd clients/javascript && npm test

# Python
cd clients/python && pytest

# Go
cd clients/go && go test ./...
```

### Build All Clients

```bash
# JavaScript
cd clients/javascript && npm run build

# Python
cd clients/python && python -m build

# Go
cd clients/go && go build ./...
```

---

## Troubleshooting

### npm: Package name already exists

- Use scoped package: `@mantisdb/client`
- Or choose different name: `mantisdb-js-client`

### PyPI: Package name already taken

- Choose different name: `mantisdb-python`, `pymantisdb`
- Request name transfer if abandoned

### Go: Module not found

- Ensure repository is public
- Wait a few minutes for indexing
- Trigger manual indexing with curl command

### Authentication Issues

- Use API tokens instead of passwords
- Store tokens in GitHub Secrets
- Never commit tokens to repository

---

## Support

For publishing issues:
- **npm**: https://docs.npmjs.com/
- **PyPI**: https://packaging.python.org/
- **Go**: https://go.dev/doc/modules/publishing
- **GitHub**: https://github.com/mantisdb/mantisdb/issues
