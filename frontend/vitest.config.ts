import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/vitestSetup.ts'],
    // jsdom plus the graph renderer exhausts the 2 GB runner when Vitest forks every file.
    // Two threads preserve useful parallelism without exceeding the constrained runner heap.
    pool: 'threads',
    maxWorkers: 2,
  },
})
