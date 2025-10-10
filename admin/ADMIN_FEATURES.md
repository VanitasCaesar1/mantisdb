# MantisDB Admin Dashboard - Feature Guide

## Overview
The MantisDB Admin Dashboard is a comprehensive, Supabase-like interface for managing your MantisDB instance. It provides full CRUD operations, real-time monitoring, authentication, and advanced configuration management.

## 🔐 Authentication System

### Login Credentials
- **Default Email**: `admin@mantisdb.io`
- **Default Password**: `admin123`

### Features
- JWT-based session management
- Secure token storage in localStorage
- Automatic session expiration (24 hours)
- Protected routes - unauthenticated users are redirected to login
- User info displayed in header with logout button

### API Endpoints
- `POST /api/auth/login` - Authenticate and receive token
- `GET /api/auth/verify` - Verify token validity
- `POST /api/auth/logout` - End session

## 📊 Enhanced Data Browser

### Features
- **View Tables/Collections**: Browse all tables, collections, and key-value stores
- **Full CRUD Operations**:
  - ✅ **Create**: Add new rows with a modal form
  - ✅ **Read**: View and paginate through data
  - ✅ **Update**: Click any cell to edit inline
  - ✅ **Delete**: Delete rows with confirmation
- **Search & Filter**: Real-time search across all columns
- **Pagination**: Navigate large datasets efficiently
- **Type Detection**: Automatically handles tables, collections, and key-value data
- **Cell Editing**: Click to edit, Enter to save, Escape to cancel

### Usage
1. Select a table/collection from the grid
2. View data in spreadsheet-like interface
3. Click any cell to edit
4. Use "Add Row" button to insert new records
5. Hover over rows to see delete button
6. Use search box for quick filtering

### API Endpoints
- `GET /api/tables` - List all tables
- `GET /api/tables/{name}?type={type}&limit={n}&offset={n}` - Get table data
- `POST /api/tables/{name}?type={type}` - Create new row
- `PUT /api/tables/{name}/data/{id}?type={type}` - Update row
- `DELETE /api/tables/{name}/data/{id}?type={type}` - Delete row

## ⚙️ Enhanced Settings

### Settings Tabs

#### 1. General (Server Configuration)
- **Host**: Server bind address
- **Main Port**: API server port (default: 8081)
- **Admin Port**: Admin interface port (default: 8082)

#### 2. Database Configuration
- **Data Directory**: Path to database storage
- **Cache Size**: Memory cache allocation (MB)
- **Max Connections**: Concurrent connection limit
- **WAL Toggle**: Enable/disable Write-Ahead Logging

#### 3. Security Configuration
- **Authentication Toggle**: Enable/disable auth requirement
- **Session Timeout**: Auto-logout duration (seconds)
- **Max Login Attempts**: Failed login threshold

#### 4. Performance Configuration
- **Query Timeout**: Max query execution time (seconds)
- **Max Query Size**: Maximum query payload (bytes)
- **Query Cache Toggle**: Enable result caching

#### 5. Backup Configuration
- **Auto Backup**: Enable periodic backups
- **Backup Interval**: Hours between backups
- **Retention Period**: Days to keep backups

#### 6. Features
- Toggle various feature flags
- Dynamic feature management

### API Endpoints
- `GET /api/config` - Get current configuration
- `PUT /api/config` - Update configuration

## 🎨 UI/UX Improvements

### Design
- **Modern Gradient Login**: Beautiful landing page
- **Responsive Layout**: Works on desktop, tablet, and mobile
- **Tab Navigation**: Organized settings with icons
- **Live Status Indicators**: Connection status, online/offline badges
- **Smooth Transitions**: Hover effects and animations
- **Color-Coded Types**: Different colors for tables, collections, key-value stores

### Components
- **Enhanced Cards**: Clean, bordered sections
- **Modal Dialogs**: For add/edit operations
- **Toast Messages**: Success/error notifications
- **Loading States**: Spinners for async operations
- **Empty States**: Helpful messages when no data exists

## 📈 Dashboard Features

### Real-Time Metrics
- Total Records counter
- Active Connections counter
- CPU Usage percentage
- Memory Usage display
- Live Metrics with WebSocket updates

### System Status
- Database Engine health
- Version information
- Uptime tracking
- Platform details

### Performance Monitoring
- Queries per second
- Cache hit ratio
- Average response time
- Real-time updates via SSE

## 🔧 Technical Stack

### Frontend
- **React 18** with TypeScript
- **Tailwind CSS** for styling
- **Custom Hooks** for API integration
- **Context API** for auth state
- **Server-Sent Events** for real-time data

### Backend
- **Go** with standard library
- **RESTful API** design
- **Session-based auth** with tokens
- **CORS enabled** for development
- **Mock data** for demonstration

## 🚀 Getting Started

### 1. Start the Backend
```bash
cd admin
go run ../cmd/mantisDB/main.go
```

### 2. Start the Frontend
```bash
cd admin/frontend
npm install
npm run dev
```

### 3. Access Dashboard
Open browser to `http://localhost:5173` (or your Vite dev port)

### 4. Login
- Email: `admin@mantisdb.io`
- Password: `admin123`

## 🔒 Security Notes

### Production Recommendations
1. **Change Default Credentials**: Update default admin password
2. **Use HTTPS**: Enable TLS for production
3. **Hash Passwords**: Implement bcrypt for password storage (currently plaintext for demo)
4. **JWT Secrets**: Use environment variables for signing keys
5. **Rate Limiting**: Add rate limiting to prevent brute force
6. **CORS**: Restrict origins in production
7. **Session Storage**: Use Redis or database for sessions
8. **Audit Logging**: Track all admin actions

## 📝 Comparison with Supabase

### What We Have
✅ Authentication system
✅ Data browser with full CRUD
✅ Inline cell editing
✅ Row insertion/deletion
✅ Real-time monitoring
✅ Comprehensive settings
✅ Search and filtering
✅ Pagination
✅ Modern UI/UX
✅ Role-based access

### Additional Features
- Query editor with history
- Backup management
- Log viewer with streaming
- System health monitoring
- WebSocket support
- Prometheus metrics export

## 🐛 Known Limitations

1. **Authentication**: Simple token-based (not production-ready without enhancements)
2. **Data Types**: Limited type inference for cell editing
3. **Validation**: Basic validation, needs enhancement
4. **Permissions**: No fine-grained permissions yet
5. **Audit Trail**: No audit logging implemented

## 📚 API Documentation

### Authentication
```bash
# Login
curl -X POST http://localhost:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@mantisdb.io","password":"admin123"}'

# Verify token
curl http://localhost:8081/api/auth/verify \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Data Operations
```bash
# List tables
curl http://localhost:8081/api/tables

# Get table data
curl "http://localhost:8081/api/tables/users?type=collection&limit=50&offset=0"

# Create row
curl -X POST http://localhost:8081/api/tables/users?type=collection \
  -H "Content-Type: application/json" \
  -d '{"name":"John Doe","email":"john@example.com"}'

# Update row
curl -X PUT http://localhost:8081/api/tables/users/data/123?type=collection \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane Doe","email":"jane@example.com"}'

# Delete row
curl -X DELETE http://localhost:8081/api/tables/users/data/123?type=collection
```

## 🎯 Future Enhancements

1. **SQL Query Builder**: Visual query builder
2. **Schema Designer**: Visual table/collection designer
3. **API Generator**: Auto-generate REST/GraphQL APIs
4. **Collaboration**: Multi-user with presence
5. **Webhooks**: Event-driven integrations
6. **Migrations**: Database migration management
7. **Replication**: Master-slave configuration UI
8. **Monitoring Alerts**: Configurable alerting
9. **Export/Import**: CSV, JSON, SQL export
10. **Documentation**: Auto-generated API docs

## 🤝 Contributing

To add new features:
1. Add backend endpoint in `admin/api/server.go`
2. Create/update frontend component in `admin/frontend/src/components/`
3. Add API hook in `admin/frontend/src/hooks/useApi.ts`
4. Test with real MantisDB instance
5. Update this documentation

## 📞 Support

For issues or questions:
- Check existing GitHub issues
- Create new issue with detailed description
- Include browser console logs for UI issues
- Include server logs for backend issues

---

**Version**: 1.0.0  
**Last Updated**: 2025-10-07  
**License**: MIT
