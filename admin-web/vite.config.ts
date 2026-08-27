import { defineConfig } from 'vitest/config'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'

export default defineConfig({ plugins: [tailwindcss(), react()], test: { environment: 'jsdom', exclude: ['**/node_modules/**', '**/dist/**'] } })
