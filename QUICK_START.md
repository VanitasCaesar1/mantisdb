# MantisDB Quick Start Guide

## 🚀 Running MantisDB (Fixed!)

The admin UI build was getting stuck due to file system timeouts. Here's the **simple solution**:

### **Option 1: Run Without Building** (Recommended - Fastest)

Open **two terminals**:

**Terminal 1 - Backend:**
```bash
make run-backend
```

**Terminal 2 - Dashboard:**
```bash
make run-dashboard
```

Then open: **http://localhost:5173**

---

### **Option 2: Use the Helper Script**

```bash
./run-admin-dashboard.sh
```

This script automatically starts both backend and frontend.

---

### **Option 3: Manual Commands**

**Terminal 1 - Backend:**
```bash
go run cmd/mantisDB/main.go
```

**Terminal 2 - Dashboard:**
```bash
cd admin/frontend
npm install  # First time only
npm run dev
```

Then open: **http://localhost:5173**

---

## 🎯 Why Was `make run` Getting Stuck?

The issue was in the build process:
1. `make run` calls `build-unified.sh`
2. `build-unified.sh` tries to build the admin UI with `npm run build`
3. TypeScript + Vite build was hitting file system timeouts
4. **Solution**: Skip the build and run the dev server directly

## ✅ What's Fixed

- **API Configuration**: Vite proxy now correctly points to port 8080
- **Performance**: React components now use memoization
- **Backend**: Real metrics instead of mock data
- **Build Process**: New Makefile targets to skip problematic UI build

## 📚 Available Commands

```bash
make help              # Show all available commands
make run-backend       # Start backend only
make run-dashboard     # Start dashboard dev server
make dev              # Run backend in dev mode
make clean            # Clean build artifacts
```

## 🔧 Troubleshooting

### Port Already in Use
```bash
lsof -ti:8080 | xargs kill -9  # Kill backend
lsof -ti:5173 | xargs kill -9  # Kill dashboard
```

### Clear Node Modules
```bash
cd admin/frontend
rm -rf node_modules package-lock.json
npm install
```

### Backend Not Responding
```bash
# Test the backend
curl http://localhost:8080/health
```

## 🎨 Dashboard Features

Once running, you can access:
- **Dashboard**: System stats and metrics
- **Table Editor**: Browse and edit data
- **SQL Editor**: Run SQL queries
- **Monitoring**: Real-time metrics
- **Authentication**: User management
- **API Docs**: Interactive API documentation

---

**Next Steps**: Run `make run-backend` and `make run-dashboard` in separate terminals!
