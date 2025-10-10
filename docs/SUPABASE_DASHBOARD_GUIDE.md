# MantisDB Supabase-Style Dashboard - Complete Guide

## 🎯 Overview

This guide covers the complete implementation of a Supabase-inspired admin dashboard for MantisDB, including Row Level Security (RLS), table management, SQL editing, and comprehensive monitoring.

## 📦 What's Been Implemented

### 1. **Row Level Security (RLS) System**

#### Rust Core Implementation
**Location:** `rust-core/src/rls.rs`

- PostgreSQL-compatible RLS engine
- Policy types: SELECT, INSERT, UPDATE, DELETE, ALL
- Permission models: Permissive (OR logic) and Restrictive (AND logic)
- Expression evaluation with optimization
- Role-based access control

**Key Features:**
```rust
// Enable RLS for a table
engine.enable_rls("users")?;

// Add a policy
let policy = Policy {
    name: "users_select_own".to_string(),
    table: "users".to_string(),
    command: PolicyCommand::Select,
    permission: PolicyPermission::Permissive,
    roles: vec!["authenticated".to_string()],
    using_expr: Some("user_id = auth.uid()".to_string()),
    enabled: true,
};
engine.add_policy(policy)?;

// Check permissions
let allowed = engine.check_select("users", &context, &row_data)?;
```

#### Go Integration
**Location:** `rls/rls.go`, `rls/handlers.go`

- FFI bindings to Rust RLS engine
- HTTP API endpoints for policy management
- Context-aware permission checking

**API Endpoints:**
- `POST /api/rls/enable` - Enable RLS for a table
- `POST /api/rls/disable` - Disable RLS for a table
- `GET /api/rls/status` - Check RLS status
- `POST /api/rls/policies/add` - Add a new policy
- `POST /api/rls/policies/remove` - Remove a policy
- `GET /api/rls/policies` - List policies for a table
- `POST /api/rls/check` - Check if operation is allowed

### 2. **Table Editor Component**
**Location:** `admin/frontend/src/components/table-editor/TableGrid.tsx`

Features:
- Excel-like grid interface
- Inline cell editing
- Row selection and bulk operations
- Pagination with configurable page size
- Add/delete rows
- Real-time data updates

### 3. **SQL Editor Component**
**Location:** `admin/frontend/src/components/sql-editor/SQLEditor.tsx`

Features:
- Monaco Editor integration (VS Code editor)
- Syntax highlighting for SQL
- Auto-completion
- Query execution with Ctrl+Enter
- Results table with export to CSV
- Query history tracking
- Error display with stack traces
- Execution time metrics

### 4. **RLS Policy Manager**
**Location:** `admin/frontend/src/components/rls/RLSPolicyManager.tsx`

Features:
- Visual policy management interface
- Enable/disable RLS per table
- Create policies with expression builder
- Policy templates
- Role assignment
- USING and WITH CHECK expressions
- Policy testing interface

### 5. **Schema Visualizer**
**Location:** `admin/frontend/src/components/schema/SchemaVisualizer.tsx`

Features:
- Visual database schema display
- Table list with row counts
- Column details with types and constraints
- Primary key and nullable indicators
- Schema diagram visualization

### 6. **Authentication Management**
**Location:** `admin/frontend/src/components/auth/AuthManagement.tsx`

Features:
- User list and management
- Create new users with roles
- Role-based access control
- User deletion
- Session management

### 7. **Storage Manager**
**Location:** `admin/frontend/src/components/storage/StorageManager.tsx`

Features:
- File browser interface
- Multi-file upload
- File/folder management
- Storage statistics
- Download and delete operations

## 🚀 Getting Started

### Prerequisites

```bash
# Rust toolchain
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Go 1.20+
# Node.js 18+
# npm or yarn
```

### Building the Project

#### 1. Build Rust Core with RLS

```bash
cd rust-core
cargo build --release

# The library will be at: target/release/libmantisdb_core.a
# Copy to lib directory
cp target/release/libmantisdb_core.a ../lib/
```

#### 2. Build Go Backend

```bash
# From project root
go build -o mantisdb ./cmd/mantisDB/
```

#### 3. Build Admin Frontend

```bash
cd admin/frontend

# Install dependencies
npm install

# Development mode
npm run dev

# Production build
npm run build
```

### Running the System

#### 1. Start MantisDB Server

```bash
./mantisdb
# Listens on :8080 for database operations
```

#### 2. Start Admin API Server

```bash
# The admin API is integrated into the main server
# Listens on :8081 for admin operations
```

#### 3. Start Frontend Dev Server

```bash
cd admin/frontend
npm run dev
# Opens at http://localhost:5173
```

### Default Credentials

```
Email: admin@mantisdb.io
Password: admin123
```

**⚠️ Change these in production!**

## 📖 Usage Examples

### Example 1: Setting Up RLS for a Users Table

```sql
-- 1. Enable RLS
-- Via API: POST /api/rls/enable {"table": "users"}

-- 2. Create a policy that allows users to see only their own data
-- Via API: POST /api/rls/policies/add
{
  "policy": {
    "name": "users_select_own",
    "table": "users",
    "command": "Select",
    "permission": "Permissive",
    "roles": ["authenticated"],
    "using_expr": "user_id = auth.uid()",
    "enabled": true
  }
}

-- 3. Create a policy for inserting new users
{
  "policy": {
    "name": "users_insert_own",
    "table": "users",
    "command": "Insert",
    "permission": "Permissive",
    "roles": ["authenticated"],
    "with_check_expr": "user_id = auth.uid()",
    "enabled": true
  }
}
```

### Example 2: Using the SQL Editor

1. Navigate to **SQL Editor** in the sidebar
2. Write your query:
```sql
SELECT id, name, email, created_at
FROM users
WHERE created_at > '2024-01-01'
ORDER BY created_at DESC
LIMIT 100;
```
3. Press `Ctrl+Enter` or click **Execute**
4. View results in the table below
5. Export to CSV if needed

### Example 3: Managing Tables

1. Go to **Table Editor**
2. Select a table from the dropdown
3. Click cells to edit inline
4. Use **+ Add Row** to insert new records
5. Select rows and click **Delete** for bulk operations

### Example 4: Monitoring with Real-time Metrics

1. Navigate to **Monitoring** section
2. View live metrics:
   - Queries per second
   - Cache hit ratio
   - Active connections
   - CPU and memory usage
3. Metrics update every 5 seconds via Server-Sent Events

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Frontend (React + TypeScript)         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Table Editor │  │  SQL Editor  │  │ RLS Manager  │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │   Schema     │  │     Auth     │  │   Storage    │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└────────────────────────┬────────────────────────────────┘
                         │ HTTP/REST API
┌────────────────────────▼────────────────────────────────┐
│                  Go Admin API Server                     │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Admin API (server.go)                           │  │
│  │  - Table management                              │  │
│  │  - Query execution                               │  │
│  │  - Authentication                                │  │
│  │  - Monitoring & logs                             │  │
│  └───────────────────┬──────────────────────────────┘  │
│  ┌───────────────────▼──────────────────────────────┐  │
│  │  RLS Handler (rls/handlers.go)                   │  │
│  │  - Policy management                             │  │
│  │  - Permission checking                           │  │
│  └───────────────────┬──────────────────────────────┘  │
└────────────────────────┼────────────────────────────────┘
                         │ FFI (C ABI)
┌────────────────────────▼────────────────────────────────┐
│                  Rust Core (High Performance)            │
│  ┌──────────────────────────────────────────────────┐  │
│  │  RLS Engine (rls.rs)                             │  │
│  │  - Policy evaluation                             │  │
│  │  - Expression compilation                        │  │
│  │  - Context-aware checks                          │  │
│  └──────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Storage Engine (storage.rs)                     │  │
│  │  - Lock-free data structures                     │  │
│  │  - High-performance operations                   │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## 🔒 Security Considerations

### RLS Best Practices

1. **Always enable RLS for sensitive tables:**
```go
engine.EnableRLS("users")
engine.EnableRLS("private_data")
```

2. **Use restrictive policies for critical operations:**
```json
{
  "permission": "Restrictive",
  "using_expr": "role = 'admin' AND verified = true"
}
```

3. **Test policies thoroughly:**
```bash
curl -X POST http://localhost:8081/api/rls/check \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "operation": "select",
    "context": {"role": "user", "user_id": "123"},
    "row_data": {"id": 1, "user_id": "123"}
  }'
```

### Authentication

- Store JWT tokens securely
- Implement token rotation
- Use HTTPS in production
- Hash passwords with bcrypt (not implemented in demo)

## 📊 Performance Optimization

### RLS Performance

- Policies are compiled and cached
- Expression evaluation is optimized
- Lock-free data structures in Rust
- Minimal overhead per check

### Frontend Performance

- Virtual scrolling for large tables
- Debounced API calls
- React.memo for expensive components
- Code splitting with React.lazy

## 🐛 Troubleshooting

### RLS Not Working

1. Check if RLS is enabled:
```bash
curl http://localhost:8081/api/rls/status?table=users
```

2. Verify policies exist:
```bash
curl http://localhost:8081/api/rls/policies?table=users
```

3. Test permission check:
```bash
curl -X POST http://localhost:8081/api/rls/check \
  -d '{"table":"users","operation":"select","context":{"role":"user"},"row_data":{"id":1}}'
```

### Build Errors

**Rust linking errors:**
```bash
# Ensure lib directory exists
mkdir -p lib

# Rebuild Rust core
cd rust-core
cargo clean
cargo build --release
cp target/release/libmantisdb_core.a ../lib/
```

**Go CGO errors:**
```bash
# Set CGO flags
export CGO_LDFLAGS="-L./lib -lmantisdb_core"
export CGO_ENABLED=1
go build
```

## 📝 API Reference

### RLS Endpoints

#### Enable RLS
```http
POST /api/rls/enable
Content-Type: application/json

{
  "table": "users"
}
```

#### Add Policy
```http
POST /api/rls/policies/add
Content-Type: application/json

{
  "policy": {
    "name": "policy_name",
    "table": "table_name",
    "command": "Select|Insert|Update|Delete|All",
    "permission": "Permissive|Restrictive",
    "roles": ["role1", "role2"],
    "using_expr": "expression",
    "with_check_expr": "expression",
    "enabled": true
  }
}
```

#### Check Permission
```http
POST /api/rls/check
Content-Type: application/json

{
  "table": "users",
  "operation": "select",
  "context": {
    "user_id": "123",
    "role": "authenticated"
  },
  "row_data": {
    "id": 1,
    "user_id": "123"
  }
}
```

## 🎓 Learning Resources

- [PostgreSQL RLS Documentation](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [Supabase RLS Guide](https://supabase.com/docs/guides/auth/row-level-security)
- [Rust FFI Guide](https://doc.rust-lang.org/nomicon/ffi.html)
- [Monaco Editor API](https://microsoft.github.io/monaco-editor/api/index.html)

## 🤝 Contributing

See the main README.md for contribution guidelines.

## 📄 License

See LICENSE file in the project root.

---

**Built with ❤️ for MantisDB**
