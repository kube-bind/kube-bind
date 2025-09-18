#!/bin/bash

# Build frontend script for kube-bind
# This script builds the Vue.js frontend and embeds it into the Go binary

set -e

echo "Building frontend..."

# Navigate to web directory
cd web

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "Installing frontend dependencies..."
    npm install
fi

# Build the frontend
echo "Running dev application..."
npm run dev
