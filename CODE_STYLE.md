# MantisDB Code Style Guide

This project follows the principles articulated by Linus Torvalds: **clarity over cleverness, why over what, and simplicity over abstraction**.

## Core Philosophy

### Comments Explain Why, Not What
```go
// Bad: increment counter
count++

// Good: account for retry attempts in final metrics
count++
```

Code should be self-documenting. Comments explain the reasoning, edge cases, and non-obvious decisions.

### Tabs for Indentation
We use tabs for indentation across the codebase. Tabs let developers choose their visual width while maintaining consistency in version control.

### Simple is Better Than Clever
If you need comments to explain how your code works, rewrite it to be simpler. Reserve comments for explaining *why* the code exists or why a particular approach was chosen.

### Limit Abstraction Layers
Don't add abstraction for potential future use. Add it when you have at least 3 concrete use cases. Premature abstraction is harder to remove than to add later.

## Language-Specific Guidelines

### Go

Follow standard Go conventions:
- Use `gofmt` for all code
- Package comments above `package` declaration
- Exported functions/types need doc comments
- Use tabs for indentation (enforced by gofmt)
- Line length: aim for ~80 columns, hard limit at 120

```go
// Package storage implements the core persistence layer.
// It handles write-ahead logging, page management, and crash recovery.
package storage

// Open initializes a storage engine.
// We fsync the directory here to ensure the DB file is durably created
// before returning - critical for crash consistency.
func Open(path string) (*Engine, error) {
	// implementation
}
```

### Rust

Follow Rust idioms with rustfmt:
- Use `rustfmt` and address `clippy` warnings
- Module docs with `//!` at file top
- Explain `unsafe` blocks thoroughly
- Tabs for indentation (configured in rustfmt.toml)
- Document panic conditions

```rust
//! Buffer pool manages in-memory pages with LRU eviction.
//!
//! We use a lock-free hash table for lookups because contention
//! profiling showed the page table mutex was a bottleneck under
//! high concurrency (see benchmark results in docs/perf/).

/// Pins a page in memory, preventing eviction.
///
/// # Safety
/// Caller must ensure page_id is valid. Invalid IDs cause undefined
/// behavior because we index directly into the page array for speed.
pub unsafe fn pin_page(&self, page_id: PageId) -> &Page {
    // implementation
}
```

### TypeScript/JavaScript

- 4 spaces for indentation (tabs don't work well with JSX)
- Use ESLint with project config
- JSDoc for exported functions
- Prefer `const` over `let`, avoid `var`

```typescript
/**
 * Executes a SQL query with automatic retry on transient errors.
 * 
 * We retry only on network errors, not semantic SQL errors, to avoid
 * amplifying bad queries during outages.
 */
export async function executeQuery(sql: string): Promise<ResultSet> {
	// implementation
}
```

### Python

- 4 spaces (PEP 8 standard)
- Use `ruff` for linting
- Google-style docstrings
- Type hints for public APIs

```python
def execute_batch(queries: list[str], timeout_ms: int = 5000) -> list[Result]:
    """Execute multiple queries in a single round-trip.
    
    Args:
        queries: SQL statements to execute
        timeout_ms: Per-query timeout. We enforce per-query (not total) 
                   because long-running analytics queries shouldn't 
                   starve short transactional ones.
    
    Returns:
        Results in the same order as input queries.
    """
    # implementation
```

## What to Comment

### Always Comment
- **Why** a particular algorithm or data structure was chosen
- Non-obvious performance optimizations
- Workarounds for external bugs
- Unsafe code and invariants
- Lock ordering and deadlock prevention
- Error handling strategy

### Never Comment
- What the code does (if it's obvious)
- Commented-out code (delete it - Git remembers)
- TODO without assignee and date
- Obvious getter/setter behavior

## File Organization

### File Headers
Every source file starts with a brief comment explaining its purpose:

```go
// storage_engine.go - Core persistence layer with WAL and crash recovery
```

```rust
//! Transaction manager with MVCC and deadlock detection.
```

No copyright boilerplate, no change history (Git tracks that), no author names (Git tracks that too).

## Examples of Good vs. Bad

### Bad
```go
// Process data
func Process(data []byte) error {
	// Check if data is not nil
	if data == nil {
		return errors.New("nil data")
	}
	// Loop through data
	for i := 0; i < len(data); i++ {
		// Do something
		result := transform(data[i])
		// Store result
		store(result)
	}
	return nil
}
```

### Good
```go
// Process transforms and persists incoming bytes.
// We process synchronously (not batched) because the caller expects
// immediate durability guarantees per the API contract.
func Process(data []byte) error {
	if data == nil {
		return errors.New("nil data")
	}
	
	for i := 0; i < len(data); i++ {
		result := transform(data[i])
		store(result)
	}
	return nil
}
```

## Formatting Tools

Run before committing:
```bash
# Go
gofmt -w .

# Rust
cargo fmt

# TypeScript/JavaScript
npm run lint --fix

# Python
ruff format .
```

## When to Break the Rules

These are guidelines, not laws. If you have a good reason to deviate, document it in a comment.

---

*"Bad programmers worry about the code. Good programmers worry about data structures and their relationships."* — Linus Torvalds
