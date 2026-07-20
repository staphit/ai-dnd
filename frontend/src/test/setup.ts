import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

// WebGL / three.js is not available in jsdom; StoryFeed lazy-loads DMTable and
// falls back when Canvas cannot mount. Stub getContext so imports do not throw.
HTMLCanvasElement.prototype.getContext = function getContext() {
  return null;
} as typeof HTMLCanvasElement.prototype.getContext;

// Define this directly instead of reading Node's experimental localStorage
// getter, which emits a warning before jsdom has a chance to provide storage.
const values = new Map<string, string>();
Object.defineProperty(globalThis, 'localStorage', {
  configurable: true,
  value: {
    get length() { return values.size; },
    clear: () => values.clear(),
    getItem: (key: string) => values.get(key) ?? null,
    key: (index: number) => [...values.keys()][index] ?? null,
    removeItem: (key: string) => values.delete(key),
    setItem: (key: string, value: string) => values.set(key, String(value)),
  },
});

afterEach(() => cleanup());
