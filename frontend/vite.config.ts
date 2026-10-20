import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // 開発中は /api への要求を Go の API サーバーへ中継する。
    // 画面と API が同じオリジンに見えるため、API 側で CORS を許可せずに済む。
    // API を既定の 8080 以外で動かす場合は API_URL で中継先を変える。
    proxy: {
      '/api': process.env.API_URL ?? 'http://localhost:8080',
    },
  },
})
