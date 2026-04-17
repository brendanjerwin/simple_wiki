// Global type declarations for simple_wiki runtime configuration injected by the server.
interface Window {
  simple_wiki?: {
    pageName?: string;
    debounceMS?: number;
    lastFetch?: number;
    username?: string;
  };
}

// Declare as a global var so TypeScript recognises it on `globalThis` as well as `window`.
// eslint-disable-next-line no-var
declare var simple_wiki:
  | {
      pageName?: string;
      debounceMS?: number;
      lastFetch?: number;
      username?: string;
    }
  | undefined;
