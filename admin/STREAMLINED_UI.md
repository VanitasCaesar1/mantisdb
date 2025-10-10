# MantisDB Admin UI - Streamlined Version

## Overview
The admin UI has been cleaned up and streamlined to remove clutter and connect to real API endpoints instead of using mock data.

## What Changed

### 1. **Simplified Sidebar Menu**
Removed duplicate and non-functional menu items. The sidebar now has only essential, working sections:

- **Dashboard** - System overview with real-time metrics
- **Table Editor** - Browse and edit columnar tables (connected to real API)
- **SQL Editor** - Execute SQL queries
- **Monitoring** - Real-time system monitoring
- **Logs** - View system logs
- **Backups** - Manage database backups
- **Settings** - Configure MantisDB

**Removed duplicate items:**
- Multiple "Data Browser" entries (consolidated into one "Table Editor")
- Separate Key-Value, Document, Columnar browsers (will be unified in future)
- RLS Policies, Authentication, Storage, Schema sections (not yet implemented)
- Account section (redundant with header user menu)

### 2. **Connected to Real APIs**
The Table Editor (Data Browser) now:
- ✅ Fetches real table list from `/api/columnar/tables`
- ✅ Loads real table data from `/api/columnar/tables/{name}/query`
- ✅ Inserts rows via `/api/columnar/tables/{name}/rows`
- ✅ Deletes rows via `/api/columnar/tables/{name}/delete`
- ✅ Shows proper error messages when API is unavailable
- ❌ No more mock/sample data fallbacks

### 3. **Improved Error Handling**
- Clear error messages when MantisDB is not running
- Proper error display for failed API calls
- User-friendly messages for empty states

## How to Use

### Starting the System

1. **Start MantisDB Backend:**
   ```bash
   cd /Users/vanitascaesar/Documents/Projects/mantisdb
   cargo run --bin mantisdb
   ```
   This starts both the main API (port 8081) and admin API (Rust-based).

2. **Start Admin UI Frontend:**
   ```bash
   cd admin/frontend
   npm install  # First time only
   npm run dev
   ```
   This starts the Vite dev server (usually port 3000 or 5173).

3. **Access Admin UI:**
   Open browser to `http://localhost:3000` (or the port shown by Vite)

### Using the Table Editor

1. **View Tables:**
   - Click "Table Editor" in the sidebar
   - Tables are loaded from the database automatically
   - If no tables exist, you'll see a message to create one

2. **Browse Table Data:**
   - Click on a table name in the left sidebar
   - Data loads automatically with pagination
   - Use the search box to filter rows

3. **Insert Rows:**
   - Click "Insert Row" button
   - A new empty row will be created
   - (Note: Edit functionality coming soon)

4. **Delete Rows:**
   - Hover over a row
   - Click the trash icon in the Actions column
   - Confirm deletion

### API Proxy Configuration

The Vite dev server proxies API requests to the backend:
- Frontend: `http://localhost:3000`
- Backend API: `http://localhost:8081`
- Proxy: `/api/*` → `http://localhost:8081/api/*`

## Architecture

### Frontend Stack
- **React 18** with TypeScript
- **Tailwind CSS** for styling
- **Vite** for dev server and build
- **Custom hooks** for API integration

### Backend APIs
- **Rust (Axum)** - Admin API endpoints
- **Go** - Main database API
- **RESTful** design with JSON responses

### API Endpoints Used

#### Columnar Store
```
GET    /api/columnar/tables              - List all tables
POST   /api/columnar/tables              - Create table
GET    /api/columnar/tables/{name}       - Get table metadata
DELETE /api/columnar/tables/{name}       - Drop table
POST   /api/columnar/tables/{name}/rows  - Insert rows
POST   /api/columnar/tables/{name}/query - Query rows
POST   /api/columnar/tables/{name}/delete - Delete rows
```

#### System
```
GET /api/health                          - Health check
GET /api/stats                           - System statistics
GET /api/metrics                         - Performance metrics
```

## Known Limitations

1. **Edit Functionality:** Cell editing is not yet implemented
2. **Table Creation:** UI for creating new tables not yet added
3. **Data Models:** Key-Value and Document stores not yet integrated
4. **Authentication:** Simple token-based auth (not production-ready)
5. **Validation:** Limited input validation

## Next Steps

### Immediate Priorities
1. Add inline cell editing for table data
2. Add table creation UI with column definitions
3. Implement update API calls
4. Add data type validation

### Future Enhancements
1. Unified data browser for all data models (KV, Document, Columnar)
2. SQL query builder with syntax highlighting
3. Real-time data updates via WebSocket
4. Export/import functionality (CSV, JSON)
5. Advanced filtering and sorting
6. Bulk operations

## Troubleshooting

### "Failed to connect to database"
- Ensure MantisDB backend is running on port 8081
- Check if the Rust admin API is compiled and running
- Verify proxy configuration in `vite.config.ts`

### "No tables found"
- This is normal if you haven't created any tables yet
- Use the SQL Editor or API to create tables
- Example: 
  ```sql
  CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT,
    email TEXT
  );
  ```

### Tables load but show no data
- Tables may be empty (no rows inserted yet)
- Use "Insert Row" to add data
- Or use SQL: `INSERT INTO users VALUES (1, 'John', 'john@example.com');`

## Development

### File Structure
```
admin/frontend/src/
├── components/
│   ├── data-browser/
│   │   └── DataBrowser.tsx      # Main table editor component
│   ├── layout/
│   │   ├── Layout.tsx           # Main layout wrapper
│   │   ├── Sidebar.tsx          # Navigation sidebar
│   │   └── Header.tsx           # Top header bar
│   ├── sections/                # Feature sections
│   └── ui/                      # Reusable UI components
├── hooks/
│   └── useApi.ts                # API integration hooks
├── api/
│   └── client.ts                # API client
└── App.tsx                      # Main app component
```

### Adding New Features

1. **Add API endpoint** in Rust (`rust-core/src/admin_api/`)
2. **Add API client method** in `admin/frontend/src/api/client.ts`
3. **Create/update component** in `admin/frontend/src/components/`
4. **Add to sidebar** in `App.tsx` if needed

## Production Deployment

⚠️ **Not production-ready yet!** This is a development version.

Before production:
1. Implement proper authentication and authorization
2. Add HTTPS/TLS support
3. Enable CORS restrictions
4. Add rate limiting
5. Implement audit logging
6. Add input validation and sanitization
7. Build optimized frontend bundle
8. Set up proper error monitoring

---

**Version:** 2.0.0-streamlined  
**Last Updated:** 2025-10-08  
**Status:** Development
