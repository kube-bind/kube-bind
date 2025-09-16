# Kube Bind Frontend

A Vue.js + TypeScript frontend application for the Kube Bind project that provides a web interface for binding Kubernetes resources across clusters with SSO authentication.

## Features

- 🔐 **SSO Authentication**: OAuth2/OIDC-based authentication via `/api/authorize` endpoint
- 🔗 **Resource Management**: Browse and bind available Kubernetes resources
- ⚡ **Modern Stack**: Built with Vue.js 3, TypeScript, and Vite
- 📱 **Responsive Design**: Works on desktop and mobile devices

## Architecture

The frontend integrates with the existing Go backend through the following endpoints:

- `/api/authorize` - SSO authentication endpoint
- `/api/callback` - OAuth2 callback handler
- `/api/resources` - Fetch available resources
- `/api/bind` - Bind resources to cluster

## Development Setup

### Prerequisites

- Node.js 18+ and npm
- Go 1.19+ for running the backend server

### Development Workflow

#### Option 1: Integrated Development (Recommended)
```bash
# Build the frontend first
cd web && npm install && npm run build && cd ..

# Run the Go backend server (serves both API and frontend)
go run ./cmd/backend --listen-port=8080

# Visit http://localhost:8080 for the complete application
```

#### Option 2: Separate Development Servers
```bash
# Terminal 1: Start Go backend
go run ./cmd/backend --listen-port=8080

# Terminal 2: Start frontend dev server with hot reload
cd web
npm install
npm run dev

# Visit http://localhost:3000 for frontend (proxies API to :8080)
```

### Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Lint code
- `npm run type-check` - Run TypeScript type checking

## Authentication Flow

1. User clicks "Login" or accesses protected resource
2. Frontend redirects to `/api/authorize` with session parameters
3. Backend handles OAuth2 flow with configured OIDC provider
4. User is redirected back to frontend with authentication cookie
5. Frontend can now access protected endpoints

## Project Structure

```
src/
├── main.ts              # Application entry point
├── App.vue             # Root component
├── services/
│   └── auth.ts         # Authentication service
└── views/
    ├── Home.vue        # Landing page
    ├── Login.vue       # Login form
    └── Resources.vue   # Resource management
```

## Configuration

The frontend automatically detects the backend API through Vite proxy configuration. For production deployments, ensure the frontend is served from the same domain as the backend or configure CORS appropriately.

## Building for Production

### Integrated Build
```bash
# Use the build script (builds frontend + Go binary)
./scripts/build-frontend.sh

# Or build manually:
cd web && npm run build && cd ..
go build -o bin/kube-bind-server ./cmd/backend

# The frontend is automatically embedded in the Go binary
```

### Frontend Only
```bash
cd web
npm run build
```

The built files will be in the `web/dist/` directory and are automatically embedded into the Go binary via the `//go:embed` directive.

## Architecture Integration

The frontend is now fully integrated with the Go backend:

- **Production**: Frontend assets are embedded in the Go binary using `//go:embed`
- **Development**: Fallback to local filesystem for live development
- **Routing**: SPA routes are handled by the Go server with proper fallback
- **API**: All `/api/*` routes are handled by Go, everything else serves the Vue.js app