# Kube Bind Frontend

A Vue.js + TypeScript frontend application for the Kube Bind project that provides a web interface for binding Kubernetes resources across clusters with SSO authentication.

## Features

- **SSO Authentication**: OAuth2/OIDC-based authentication via `/api/authorize` endpoint
- **Resource Management**: Browse and bind available Kubernetes resources
- **Modern Stack**: Built with Vue.js 3, TypeScript, and Vite

## Architecture

The frontend integrates with the existing Go backend and provides compatibility for CLI clients through redirect handling:

### API Endpoints

- `/api/authorize` - SSO authentication endpoint
- `/api/callback` - OAuth2 callback handler  
- `/api/resources` - Fetch available resources
- `/api/bind` - Bind resources to cluster
- `/api/clusters/{cluster}/resources` - Cluster-specific resources
- `/api/clusters/{cluster}/authorize` - Cluster-specific authentication
- `/api/exports` - Export binding configuration
- `/api/clusters/{cluster}/exports` - Cluster-specific exports

### Redirect Handling

For CLI compatibility, the following routes provide HTTP 302 redirects:

- `/exports` → `/api/exports`
- `/clusters/{cluster}/exports` → `/api/clusters/{cluster}/exports`

Frontend routes handle browser-based redirects for:

- `/authorize` → `/api/authorize`
- `/clusters/{cluster}/authorize` → `/api/clusters/{cluster}/authorize`
- `/callback` → `/api/callback`

## Development Setup

### Prerequisites

- Node.js 18+ and npm
- Go 1.19+ for running the backend server

### Development Workflow

#### Development with Hot Reload (Recommended)
```bash
# Terminal 1: Start Go backend
go run ./cmd/backend --listen-port=8080 --frontend http://localhost:3000

# Terminal 2: Start frontend dev server with hot reload
cd web
npm install
npm run dev

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

```text
src/
├── main.ts              # Application entry point and routing
├── App.vue             # Root component
├── services/
│   └── auth.ts         # Authentication and API service
└── views/
    └── Resources.vue   # Resource management interface
```

## Configuration

The frontend automatically detects the backend API through Vite proxy configuration. For production deployments, ensure the frontend is served from the same domain as the backend or configure CORS appropriately.

## Building for Production

### Integrated Build
```bash
# Use the build script (builds frontend + Go binary)
./scripts/build-frontend.sh
```

### Frontend Only
```bash
cd web
npm run build
```

The built files will be in the `web/dist/` directory and are automatically embedded into the container image and served from there.