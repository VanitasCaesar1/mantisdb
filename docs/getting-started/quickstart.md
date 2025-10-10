# MantisDB Dashboard - Quick Start Guide

## 🚀 Get Up and Running in 5 Minutes

### Step 1: Build Rust Core (2 minutes)

```bash
cd rust-core
cargo build --release
mkdir -p ../lib
cp target/release/libmantisdb_core.a ../lib/
cd ..
```

### Step 2: Build Go Backend (1 minute)

```bash
go build -o mantisdb ./cmd/mantisDB/
```

### Step 3: Start the Server (30 seconds)

```bash
./mantisdb
```

Server will start on:
- Database API: `http://localhost:8080`
- Admin API: `http://localhost:8081`

### Step 4: Start Frontend (1 minute)

```bash
cd admin/frontend
npm install  # First time only
npm run dev
```

Frontend will open at: `http://localhost:5173`

### Step 5: Login

```
Email: admin@mantisdb.io
Password: admin123
```

## ✨ What You Can Do Now

### 1. **View Dashboard**
- See system metrics
- Monitor database health
- View real-time statistics

### 2. **Manage Tables**
- Click **Table Editor** in sidebar
- View and edit data in spreadsheet-like interface
- Add/delete rows
- Export data

### 3. **Run SQL Queries**
- Click **SQL Editor**
- Write SQL queries with autocomplete
- Execute with `Ctrl+Enter`
- Export results to CSV

### 4. **Set Up Row Level Security**
- Click **RLS Policies**
- Select a table
- Enable RLS
- Add policies to control access

### 5. **Visualize Schema**
- Click **Schema**
- Browse tables and columns
- View relationships
- See constraints

### 6. **Manage Users**
- Click **Authentication**
- Add new users
- Assign roles
- Manage permissions

## 📋 Quick Examples

### Create a Policy

```javascript
// In RLS Policies section:
{
  "name": "users_read_own",
  "table": "users",
  "command": "Select",
  "permission": "Permissive",
  "roles": ["authenticated"],
  "using_expr": "user_id = auth.uid()",
  "enabled": true
}
```

### Run a Query

```sql
-- In SQL Editor:
SELECT * FROM users 
WHERE created_at > NOW() - INTERVAL '7 days'
ORDER BY created_at DESC
LIMIT 100;
```

### Add a Row

1. Go to **Table Editor**
2. Select your table
3. Click **+ Add Row**
4. Fill in the data
5. Changes save automatically

## 🔧 Troubleshooting

### Port Already in Use

```bash
# Kill process on port 8080
lsof -ti:8080 | xargs kill -9

# Kill process on port 8081
lsof -ti:8081 | xargs kill -9
```

### Build Errors

```bash
# Clean and rebuild
cd rust-core
cargo clean
cargo build --release

# Ensure lib exists
mkdir -p ../lib
cp target/release/libmantisdb_core.a ../lib/
```

### Frontend Won't Start

```bash
cd admin/frontend
rm -rf node_modules package-lock.json
npm install
npm run dev
```

## 📚 Next Steps

1. Read the [Complete Dashboard Guide](./docs/SUPABASE_DASHBOARD_GUIDE.md)
2. Explore the [API Documentation](./docs/API.md)
3. Check out [RLS Examples](./docs/RLS_EXAMPLES.md)

## 🆘 Need Help?

- Check the logs in `monitoring/logs/`
- View system stats at `/api/stats`
- Enable debug mode in config

---

**Happy Building! 🎉**
