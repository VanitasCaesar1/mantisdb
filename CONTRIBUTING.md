# Contributing to MantisDB

Thanks for your interest in contributing to MantisDB. This document outlines the process and guidelines.

## Code Style

We follow Linus Torvalds' philosophy on code comments:

- **Comments explain WHY, not WHAT**: The code shows what it does. Comments explain why we made certain decisions.
- **No obvious comments**: Don't comment `i++; // increment i`. That's noise.
- **Document trade-offs**: When you choose one approach over another, explain why.
- **Be direct**: No corporate speak. Write like you're explaining to a colleague.

### Example of Good Comments

```go
/*
 * We use RWMutex here instead of sync.Map because benchmarks showed
 * RWMutex is 30% faster for our read-heavy workload. sync.Map wins
 * for write-heavy workloads, but that's not our use case.
 */
```

### Example of Bad Comments

```go
// This function adds two numbers
func add(a, b int) int {
    return a + b  // return the sum
}
```

## Building and Testing

```bash
# Build everything
make build

# Run tests
make test

# Run benchmarks
make bench
```

## Pull Request Process

1. Fork the repo
2. Create a feature branch: `git checkout -b feature/your-feature`
3. Write tests for your changes
4. Ensure all tests pass: `make test`
5. Add meaningful commit messages
6. Submit a PR with a clear description

## Project Structure

```
mantisdb/
├── cmd/mantisDB/       # Main entry point
├── storage/            # Storage engines (Pure Go, Rust FFI)
├── cache/              # Caching layer with dependency tracking
├── query/              # SQL parser and query execution
├── api/                # REST API server
├── admin/              # Admin dashboard (React)
├── rust-core/          # Rust components (performance-critical code)
└── docs/               # Documentation
```

## Performance Guidelines

- **Benchmark before optimizing**: Don't guess, measure.
- **Profile hot paths**: Use `go test -bench` and `pprof`.
- **Avoid premature optimization**: Readable code first, then optimize if needed.
- **Document performance decisions**: If you choose a complex approach for speed, explain why.

## Testing Guidelines

- Write tests for new features
- Include edge cases
- Test error paths, not just happy paths
- Use table-driven tests for multiple scenarios

## Questions?

Open an issue or start a discussion. We're here to help.
