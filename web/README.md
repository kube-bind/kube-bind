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
- Running Kube Bind backend server

### Installation

```bash
# Navigate to web directory
cd web

# Install dependencies
npm install
n
# Start development server
npm run dev
```

The development server will start on `http://localhost:3000` with API proxy to `http://localhost:8080`.

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

```bash
cd web
npm run build
```

The built files will be in the `dist/` directory and can be served by any static file server or integrated into the Go backend's static file serving.