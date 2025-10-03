# MantisDB Admin Dashboard Frontend

A React-based admin dashboard for MantisDB with a nature-inspired mantis theme.

## Features

- **Mantis Theme**: Green/nature-inspired color scheme with custom Tailwind CSS configuration
- **Component Library**: Reusable UI components (Button, Card, Input, Badge, Modal)
- **Layout System**: Responsive layout with sidebar navigation and header
- **TypeScript**: Full TypeScript support with proper type definitions
- **Icons**: Custom SVG icon set for database operations
- **Utilities**: Helper functions for formatting, validation, and data manipulation

## Theme Colors

### Primary (Mantis Green)
- `mantis-50` to `mantis-950`: Main green color palette
- Primary brand color: `mantis-600` (#16a34a)

### Secondary (Forest Green)
- `forest-50` to `forest-950`: Complementary green palette
- Used for accents and secondary elements

### Usage
```tsx
// Using theme colors in components
<div className="bg-mantis-600 text-white">
  Primary button
</div>

<div className="bg-forest-100 text-forest-800">
  Secondary element
</div>
```

## Component Structure

```
src/
├── components/
│   ├── ui/           # Basic UI components
│   ├── layout/       # Layout components
│   └── icons/        # SVG icon components
├── theme/            # Theme configuration
├── types/            # TypeScript type definitions
├── utils/            # Utility functions
└── App.tsx           # Main application component
```

## Available Components

### UI Components
- `Button`: Primary, secondary, danger, and ghost variants
- `Card`: Container with header, title, and content sections
- `Input`: Form input with label, error states, and icons
- `Badge`: Status indicators with color variants
- `Modal`: Overlay dialogs with backdrop and close functionality

### Layout Components
- `Layout`: Main application layout with sidebar and content area
- `Sidebar`: Navigation sidebar with mantis branding
- `Header`: Page header with breadcrumbs and actions

### Icons
- Database, Query, Monitor, Backup, Settings, Logs, Dashboard icons
- Search, Refresh, Plus icons for actions

## Development

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Build for production
npm run build

# Type check
npm run type-check

# Lint code
npm run lint
```

## Theme Customization

The mantis theme is defined in `src/theme/index.ts` and can be customized by modifying the color palette, spacing, typography, and other design tokens.

Custom CSS classes are available:
- `.mantis-card`: Styled card component
- `.mantis-button-primary`: Primary button styling
- `.mantis-button-secondary`: Secondary button styling
- `.mantis-input`: Form input styling
- `.mantis-sidebar`: Sidebar styling
- `.mantis-nav-item`: Navigation item styling

## Integration

The frontend is designed to integrate with the MantisDB admin API server. It expects REST endpoints for:
- Data management (`/api/tables/*`)
- Query execution (`/api/query`)
- Monitoring (`/api/metrics`, `/api/health`)
- Backup management (`/api/backups/*`)
- Configuration (`/api/config`)

WebSocket connections are used for real-time updates of metrics and logs.