import type { CapacitorConfig } from '@capacitor/cli';

// Development mode is controlled by CAPACITOR_DEV environment variable
// Set CAPACITOR_DEV=true when building for local development
// For production builds, leave unset or set to false
const isDevelopment = process.env.CAPACITOR_DEV === 'true';

const config: CapacitorConfig = {
  appId: 'com.github.brendanjerwin.simple_wiki',
  appName: 'Simple Wiki',
  webDir: 'static',
  ...(isDevelopment && {
    server: {
      // In development, point to local Go server
      url: 'http://localhost:8050',
      cleartext: true
    }
  })
};

export default config;
