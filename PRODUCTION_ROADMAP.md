# MantisDB Production Readiness Roadmap

Based on the security audit conducted January 2026. This roadmap prioritizes fixes required before production deployment.

---

## Phase 1: Critical Security (Week 1-2)

### 1.1 Authentication & Authorization
- [ ] Remove "development mode" authentication bypass in `cmd/mantisDB/main.go:536-540`
- [ ] Require `MANTIS_ADMIN_TOKEN` to be set (fail startup if empty in production)
- [ ] Add authentication middleware to all `/api/v1/*` endpoints
- [ ] Implement per-endpoint API key validation
- [ ] Add role-based access control (RBAC) with user/admin roles

### 1.2 SQL Injection Prevention
- [ ] Implement parameterized query support in `pkg/sql/executor.go`
- [ ] Add identifier sanitization for table/column names
- [ ] Add query complexity limits (max depth, max joins, max subqueries)
- [ ] Audit and fix string interpolation in test files

### 1.3 Information Disclosure
- [ ] Create sanitized error responses for API layer - strip stack traces
- [ ] Add error code mapping (internal errors → safe client messages)
- [ ] Remove `getStack()` from production error paths in `internal/errors/common.go`
- [ ] Audit `/api/v1/stats` and `/api/v1/version` for sensitive data exposure

---

## Phase 2: API Security (Week 2-3)

### 2.1 Rate Limiting
- [ ] Implement rate limiting middleware using existing `RateLimit` config
- [ ] Add per-IP and per-API-key rate limits
- [ ] Add rate limiting to batch endpoint (ops/second, not just batch size)
- [ ] Implement circuit breaker for downstream failures

### 2.2 Input Validation Hardening
- [ ] Add request size limits to all endpoints (not just batch)
- [ ] Implement query timeout enforcement at HTTP layer
- [ ] Add maximum response size limits
- [ ] Validate Content-Type headers strictly

### 2.3 CORS & Headers
- [ ] Change default `CORSOrigins` from `"*"` to empty/explicit list
- [ ] Add Content-Security-Policy header
- [ ] Add Strict-Transport-Security header for HTTPS deployments
- [ ] Document required CORS configuration for production

---

## Phase 3: Data Durability (Week 3-4)

### 3.1 WAL & Sync
- [ ] Change `SyncWrites` default to `true` for production config
- [ ] Add configurable fsync strategy (every write, batched, periodic)
- [ ] Verify fsync is called before marking transactions committed
- [ ] Add WAL corruption detection on startup

### 3.2 Transaction Safety
- [ ] Fail transactions when lock release fails (not just log warning)
- [ ] Add transaction timeout enforcement
- [ ] Implement connection-level transaction cleanup on disconnect
- [ ] Add transaction leak detection and alerting

### 3.3 Recovery Validation
- [ ] Add automated crash recovery tests
- [ ] Implement recovery progress reporting
- [ ] Add checksum validation for recovered data
- [ ] Document RPO/RTO guarantees

---

## Phase 4: Operational Readiness (Week 4-5)

### 4.1 Configuration Security
- [ ] Add config file permission validation (warn if world-readable)
- [ ] Implement secret rotation support for admin tokens
- [ ] Add environment variable validation on startup
- [ ] Create production config template with secure defaults

### 4.2 Logging & Audit
- [ ] Add structured logging with log levels
- [ ] Implement audit logging for auth events (login, failed attempts)
- [ ] Add audit logging for data access (reads/writes by user)
- [ ] Implement log rotation and retention policies

### 4.3 Monitoring & Alerting
- [ ] Expose Prometheus metrics endpoint
- [ ] Add health check depth levels (shallow/deep)
- [ ] Implement alerting for security events
- [ ] Add performance baseline metrics

---

## Phase 5: Code Quality (Week 5-6)

### 5.1 Complete Implementations
- [ ] Resolve TODOs in `cmd/mantisDB/main.go`
- [ ] Complete SQL executor placeholder implementations
- [ ] Finish Columnar API handlers
- [ ] Integrate prepared statement cache with query execution

### 5.2 Error Handling
- [ ] Standardize error handling across all packages
- [ ] Add retry logic for transient failures
- [ ] Implement graceful degradation for non-critical failures
- [ ] Add error rate monitoring

### 5.3 Testing
- [ ] Add security-focused integration tests
- [ ] Implement fuzzing for SQL parser
- [ ] Add load testing with security scenarios
- [ ] Create chaos engineering tests for recovery paths

---

## Phase 6: Documentation & Compliance (Week 6-7)

### 6.1 Security Documentation
- [ ] Document authentication/authorization model
- [ ] Create security hardening guide
- [ ] Document encryption at rest/in transit options
- [ ] Add threat model documentation

### 6.2 Operational Documentation
- [ ] Create production deployment checklist
- [ ] Document backup/restore procedures
- [ ] Add disaster recovery runbook
- [ ] Create incident response procedures

### 6.3 Compliance Preparation
- [ ] Audit for PII handling compliance
- [ ] Document data retention policies
- [ ] Add audit log export capabilities
- [ ] Create compliance mapping document

---

## Production Checklist

Before going live, verify:

```
[ ] MANTIS_ADMIN_TOKEN is set to a strong random value
[ ] SyncWrites is enabled or durability trade-offs documented
[ ] Rate limiting is configured and tested
[ ] CORS origins are explicitly configured (not "*")
[ ] TLS is enabled for all endpoints
[ ] Audit logging is enabled and shipping to secure storage
[ ] Monitoring and alerting is configured
[ ] Backup procedures are tested
[ ] Recovery procedures are tested
[ ] Security scan completed with no critical/high findings
```

---

## Risk Matrix

| Issue | Severity | Effort | Priority |
|-------|----------|--------|----------|
| Auth bypass | Critical | Low | P0 |
| SQL injection | High | Medium | P0 |
| Info disclosure | Medium | Low | P1 |
| Missing rate limits | Medium | Medium | P1 |
| SyncWrites default | Medium | Low | P1 |
| Lock release failures | Medium | Low | P2 |
| Incomplete implementations | Low | High | P3 |

---

## Timeline Summary

| Phase | Duration | Focus |
|-------|----------|-------|
| Phase 1 | Week 1-2 | Critical security fixes |
| Phase 2 | Week 2-3 | API hardening |
| Phase 3 | Week 3-4 | Data durability |
| Phase 4 | Week 4-5 | Operations |
| Phase 5 | Week 5-6 | Code quality |
| Phase 6 | Week 6-7 | Documentation |

**Estimated total: 6-7 weeks** for full production readiness.

Phases 1-2 are blockers for any production deployment. Phases 3-6 can be parallelized or adjusted based on specific deployment requirements.
