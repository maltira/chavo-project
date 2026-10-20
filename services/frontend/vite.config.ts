/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'

import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'
import svgr from 'vite-plugin-svgr'

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '')
    const target = env.DEV_PROXY_TARGET || `http://localhost:${env.NGINX_PORT || 8080}`

    return {
        plugins: [react(), svgr()],
        resolve: {
            alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
        },
        css: {
            preprocessorOptions: {
                scss: {
                    additionalData: `@use "@/app/styles/typography" as *;\n@use "@/app/styles/tokens" as *;\n@use "@/app/styles/functions" as *;\n`,
                },
            },
        },
        server: {
            port: 3000,
            strictPort: true,
            proxy: {
                '/api': { target, changeOrigin: false },
                '/ws': { target, changeOrigin: false, ws: true },
                // загруженные фото (аватарки) отдаёт nginx -> Gateway
                '/media': { target, changeOrigin: false },
            },
        },
        test: {
            environment: 'jsdom',
            globals: true,
            setupFiles: ['./src/app/test-setup.ts'],
            css: false,
        },
    }
})
