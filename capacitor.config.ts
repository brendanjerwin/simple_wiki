import type { CapacitorConfig } from '@capacitor/cli';

// Development mode is controlled by CAPACITOR_DEV environment variable
// Set CAPACITOR_DEV=true when building for local development
// For production builds, leave unset or set to false
const isDevelopment = process.env.CAPACITOR_DEV === 'true';

const config: CapacitorConfig = {
  appId: 'com.github.brendanjerwin.simple_wiki',
  appName: 'Simple Wiki',
  webDir: 'static',
  server: {
    // Point to Tailscale wiki server
    url: isDevelopment ? 'http://localhost:8050' : 'https://wiki.monster-orfe.ts.net',
    cleartext: isDevelopment
  }
};

export default config;
