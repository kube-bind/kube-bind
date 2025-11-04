import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// Docker-specific configuration that avoids native dependency issues
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
    // Target ES2015 for broader compatibility
    target: 'es2015',
    // Disable minification to avoid potential native dep issues
    minify: false,
    // Use Rollup options that avoid native dependencies
    rollupOptions: {
      output: {
        // Simplified output configuration
        assetFileNames: 'assets/[name].[hash][extname]',
        chunkFileNames: 'assets/[name].[hash].js',
        entryFileNames: 'assets/[name].[hash].js',
        // Avoid code splitting to reduce complexity
        manualChunks: undefined
      }
    }
  },
  // Ensure proper base URL for production
  base: './',
  // TypeScript configuration for better compatibility
  esbuild: {
    target: 'es2015'
  },
  // Optimize dependencies to avoid potential issues
  optimizeDeps: {
    exclude: ['@rollup/rollup-linux-arm64-gnu', '@rollup/rollup-linux-x64-gnu']
  }
})