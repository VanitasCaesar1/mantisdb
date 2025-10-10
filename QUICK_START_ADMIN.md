# Quick Start: MantisDB Admin UI

## 🚀 Start in 3 Steps

### 1. Start MantisDB Backend
```bash
# From project root
cargo run --bin mantisdb
```

This starts:
- Main API on port 8081
- Admin API endpoints (Rust/Axum)

### 2. Start Admin UI (Development)
```bash
cd admin/frontend
npm install  # First time only
npm run dev
```

Access at: **http://localhost:3000**

### 3. Login
- Email: `admin@mantisdb.io`
- Password: `admin123`

---

## 📊 What You Can Do

### Dashboard
- View system metrics (CPU, memory, connections)
- Monitor database health
- See real-time performance stats

### Table Editor
- Browse all columnar tables
- View and search table data
- Insert new rows
- Delete rows
- Pagination for large datasets

### SQL Editor
- Execute SQL queries
- View query results
- Query history

### Monitoring
- Real-time metrics
- System health status
- Performance graphs

### Logs
- View system logs
- Filter by level/component
- Real-time log streaming

### Backups
- Create database backups
- Restore from backups
- Schedule automatic backups

### Settings
- Configure server settings
- Adjust performance parameters
- Manage feature flags

---

## 🔧 Creating Your First Table

### Option 1: Using SQL Editor
```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users (id, name, email) VALUES 
  (1, 'Alice', 'alice@example.com'),
  (2, 'Bob', 'bob@example.com');
```

### Option 2: Using API
```bash
# Create table
curl -X POST http://localhost:8081/api/columnar/tables \
  -H "Content-Type: application/json" \
  -d '{
    "name": "users",
    "columns": [
      {"name": "id", "data_type": "integer", "nullable": false, "primary_key": true},
      {"name": "name", "data_type": "text", "nullable": false, "primary_key": false},
      {"name": "email", "data_type": "text", "nullable": false, "primary_key": false}
    ]
  }'

# Insert rows
curl -X POST http://localhost:8081/api/columnar/tables/users/rows \
  -H "Content-Type: application/json" \
  -d '{
    "rows": [
      {"id": 1, "name": "Alice", "email": "alice@example.com"},
      {"id": 2, "name": "Bob", "email": "bob@example.com"}
    ]
  }'
```

### Option 3: Using Table Editor UI
1. Go to **Table Editor** in sidebar
2. Click **Insert Row** button
3. Fill in the row data
4. Save

---

## 🏗️ Production Build

### Build Frontend
```bash
cd admin/frontend
npm run build
```

This creates optimized files in `admin/api/assets/dist/`

### Run Production Server
```bash
# Start MantisDB with production config
cargo run --release --bin mantisdb
```

Access at: **http://localhost:8081** (serves both API and UI)

---

## 🐛 Troubleshooting

### "Failed to connect to database"
**Cause:** MantisDB backend not running  
**Fix:** Start backend with `cargo run --bin mantisdb`

### "No tables found"
**Cause:** No tables created yet  
**Fix:** Create a table using SQL Editor or API

### Port already in use
**Cause:** Another process using port 8081 or 3000  
**Fix:** 
```bash
# Find process using port
lsof -i :8081
lsof -i :3000

# Kill process
kill -9 <PID>
```

### Build errors
**Cause:** Dependencies not installed  
**Fix:**
```bash
cd admin/frontend
rm -rf node_modules package-lock.json
npm install
```

---

## 📁 Project Structure

```
mantisdb/
├── rust-core/              # Rust core engine
│   └── src/
│       └── admin_api/      # Admin API handlers
├── admin/
│   └── frontend/           # React admin UI
│       ├── src/
│       │   ├── components/ # UI components
│       │   ├── hooks/      # React hooks
│       │   └── api/        # API client
│       └── vite.config.ts  # Vite configuration
├── api/                    # Go API server
└── cmd/mantisDB/          # Main entry point
```

---

## 🔗 Useful Links

- **Admin UI:** http://localhost:3000 (dev) or http://localhost:8081 (prod)
- **API Docs:** http://localhost:8081/api/docs
- **Health Check:** http://localhost:8081/api/health
- **Metrics:** http://localhost:8081/api/metrics

---

## 📝 Next Steps

1. ✅ Create your first table
2. ✅ Insert some data
3. ✅ Browse data in Table Editor
4. ✅ Try SQL queries
5. ✅ Monitor system performance
6. ✅ Create a backup

---

## 💡 Tips

- **Auto-refresh:** Dashboard and Monitoring sections update in real-time
- **Search:** Use the search box in Table Editor to filter rows instantly
- **Pagination:** Navigate large tables with Previous/Next buttons
- **Keyboard:** Press `Ctrl+K` in SQL Editor to execute query
- **Dark Mode:** UI uses dark theme by default (matches Supabase style)

---

**Need Help?** Check `admin/STREAMLINED_UI.md` for detailed documentation.
