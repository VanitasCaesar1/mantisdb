# MantisDB SDK Publishing - Quick Summary

## Warnings Fixed ✅

1. **PostCSS Module Warning** - Added `"type": "module"` to `admin/frontend/package.json`
2. **Package Names** - Updated JavaScript client to use scoped package `@mantisdb/client`
3. **Repository Links** - Added proper repository, homepage, and bugs URLs

## Client SDKs Ready for Publishing

### 📦 JavaScript/TypeScript Client

**Package Name**: `@mantisdb/client`  
**Registry**: npm (https://www.npmjs.com)  
**Location**: `clients/javascript/`

**Quick Publish:**
```bash
cd clients/javascript
npm login
npm publish --access public
```

**Installation:**
```bash
npm install @mantisdb/client
```

---

### 🐍 Python Client

**Package Name**: `mantisdb` (or `mantisdb-python`)  
**Registry**: PyPI (https://pypi.org)  
**Location**: `clients/python/`

**Quick Publish:**
```bash
cd clients/python
python -m build
python -m twine upload dist/*
```

**Installation:**
```bash
pip install mantisdb
```

---

### 🐹 Go Client

**Package Name**: `github.com/mantisdb/mantisdb/clients/go`  
**Registry**: pkg.go.dev (automatic)  
**Location**: `clients/go/`

**Quick Publish:**
```bash
git tag clients/go/v1.0.0
git push origin clients/go/v1.0.0
```

**Installation:**
```bash
go get github.com/mantisdb/mantisdb/clients/go@v1.0.0
```

---

## Automated Publishing Script

Use the provided script to publish all clients at once:

```bash
# Dry run (test without publishing)
./scripts/publish-clients.sh --version=1.0.0 --dry-run

# Publish all clients
./scripts/publish-clients.sh --version=1.0.0

# Skip tests (faster)
./scripts/publish-clients.sh --version=1.0.0 --skip-tests
```

## Prerequisites

### For JavaScript (npm)
1. Create npm account: https://www.npmjs.com/signup
2. Login: `npm login`
3. (Optional) Create `@mantisdb` organization

### For Python (PyPI)
1. Create PyPI account: https://pypi.org/account/register/
2. Generate API token: https://pypi.org/manage/account/token/
3. Install tools: `pip install build twine`
4. Configure `~/.pypirc`:
```ini
[pypi]
username = __token__
password = pypi-YOUR_TOKEN_HERE
```

### For Go (pkg.go.dev)
1. Ensure repository is public on GitHub
2. Have push access to create tags
3. No registration needed - automatic indexing

## Step-by-Step Publishing Guide

### 1. Prepare Release

```bash
# Update version in all clients
VERSION=1.0.0

# JavaScript
sed -i '' "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" clients/javascript/package.json

# Python
sed -i '' "s/version = \".*\"/version = \"$VERSION\"/" clients/python/pyproject.toml

# Go uses git tags
```

### 2. Test Everything

```bash
# JavaScript
cd clients/javascript && npm test

# Python
cd clients/python && pytest

# Go
cd clients/go && go test ./...
```

### 3. Publish

**Option A: Use Automated Script**
```bash
./scripts/publish-clients.sh --version=1.0.0
```

**Option B: Publish Manually**

**JavaScript:**
```bash
cd clients/javascript
npm publish --access public
```

**Python:**
```bash
cd clients/python
python -m build
python -m twine upload dist/*
```

**Go:**
```bash
git tag clients/go/v1.0.0
git push origin clients/go/v1.0.0
```

### 4. Verify

**JavaScript:**
```bash
npm view @mantisdb/client
npm install @mantisdb/client
```

**Python:**
```bash
pip search mantisdb
pip install mantisdb
```

**Go:**
```bash
go get github.com/mantisdb/mantisdb/clients/go@v1.0.0
```

## GitHub Actions Automation

The repository includes GitHub Actions workflows for automated publishing:

- `.github/workflows/publish-npm.yml` - Publishes to npm on release
- `.github/workflows/publish-pypi.yml` - Publishes to PyPI on release
- `.github/workflows/publish-go.yml` - Creates Go tags on release

**Setup Required:**
1. Add `NPM_TOKEN` to GitHub Secrets
2. Add `PYPI_TOKEN` to GitHub Secrets
3. Ensure GitHub Actions has push access for tags

## Version Management

All clients follow **Semantic Versioning**:
- **MAJOR**: Breaking changes (1.0.0 → 2.0.0)
- **MINOR**: New features (1.0.0 → 1.1.0)
- **PATCH**: Bug fixes (1.0.0 → 1.0.1)

Keep client versions synchronized with MantisDB releases.

## Documentation

- **Full Publishing Guide**: `clients/PUBLISHING.md`
- **Client READMEs**: 
  - `clients/javascript/README.md`
  - `clients/python/README.md`
  - `clients/go/README.md`

## Troubleshooting

### npm: "You do not have permission to publish"
- Login: `npm login`
- Use scoped package: `@mantisdb/client`
- Check organization membership

### PyPI: "Invalid or non-existent authentication"
- Generate API token
- Update `~/.pypirc`
- Use `__token__` as username

### Go: "Module not found"
- Ensure repository is public
- Wait a few minutes for indexing
- Trigger manually: `curl https://proxy.golang.org/github.com/mantisdb/mantisdb/clients/go/@v/v1.0.0.info`

## Quick Reference

| Task | Command |
|------|---------|
| Publish all clients | `./scripts/publish-clients.sh --version=1.0.0` |
| Dry run | `./scripts/publish-clients.sh --version=1.0.0 --dry-run` |
| Publish JS only | `cd clients/javascript && npm publish` |
| Publish Python only | `cd clients/python && twine upload dist/*` |
| Publish Go only | `git tag clients/go/v1.0.0 && git push origin clients/go/v1.0.0` |
| Test all | `make build-clients` |

## Support

- **Publishing Issues**: See `clients/PUBLISHING.md`
- **GitHub Issues**: https://github.com/mantisdb/mantisdb/issues
- **npm Help**: https://docs.npmjs.com/
- **PyPI Help**: https://packaging.python.org/
- **Go Help**: https://go.dev/doc/modules/publishing

---

**Ready to publish!** 🚀

Run `./scripts/publish-clients.sh --version=1.0.0 --dry-run` to test first.
