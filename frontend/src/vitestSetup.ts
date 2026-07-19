import '@testing-library/jest-dom/vitest'

// The Cytoscape canvas lifecycle observes container resizing; jsdom has no native observer.
globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver
