import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from '@storybook/addon-actions';
import './qr-scanner.js';
import type { QrScanner, CameraProvider, CameraDevice } from './qr-scanner.js';
import { CameraPermissionError } from './qr-scanner.js';

/**
 * Mock camera provider for demonstrating error states
 */
function createMockCameraProvider(options: {
  cameras?: CameraDevice[];
  getCamerasError?: Error;
  startError?: Error;
}): CameraProvider {
  let scanning = false;

  return {
    async getCameras(): Promise<CameraDevice[]> {
      if (options.getCamerasError) {
        throw options.getCamerasError;
      }
      return options.cameras ?? [];
    },
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    async start(_v: HTMLVideoElement, _c: string, _s: (r: string) => void): Promise<void> {
      if (options.startError) {
        throw options.startError;
      }
      scanning = true;
    },
    async stop(): Promise<void> {
      scanning = false;
    },
    isScanning(): boolean {
      return scanning;
    },
  };
}

const meta: Meta = {
  title: 'Components/Inventory/QrScanner',
  tags: ['autodocs'],
  component: 'qr-scanner',
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component: `
Inline QR code scanner component for scanning QR codes using the device camera.

**Features:**
- Expandable camera viewfinder
- Automatic camera detection (prefers back camera)
- Camera selection dropdown for multiple cameras
- Emits \`qr-scanned\` event with decoded value
- Handles permission errors gracefully

**Events:**
- \`qr-scanned\`: Fired when a QR code is successfully scanned. Detail: \`{ rawValue: string }\`
- \`scanner-error\`: Fired when an error occurs. Detail: \`{ error: Error }\`

**Usage:**
\`\`\`html
<qr-scanner
  @qr-scanned=\${(e) => console.log(e.detail.rawValue)}
  @scanner-error=\${(e) => console.log(e.detail.error.message)}
></qr-scanner>
\`\`\`
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => {
    return html`
      <div style="max-width: 400px;">
        <h3>QR Scanner</h3>
        <p>Click the button below to start scanning. Requires camera access.</p>
        <qr-scanner
          @qr-scanned=${action('qr-scanned')}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Note:</strong> Camera access requires HTTPS or localhost.
          Open browser dev tools to see events.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default collapsed state. Click "Scan QR Code" to expand and start camera.',
      },
    },
  },
};

export const Expanded: Story = {
  render: () => {
    const expandScanner = () => {
      const scanner = document.querySelector('qr-scanner') as QrScanner | null;
      if (scanner) {
        scanner.expand();
      }
    };

    setTimeout(expandScanner, 200);

    return html`
      <div style="max-width: 400px;">
        <h3>Expanded Scanner</h3>
        <p>Scanner is automatically expanded on load.</p>
        <qr-scanner
          @qr-scanned=${action('qr-scanned')}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>
        <button @click=${expandScanner} style="margin-top: 10px;">Re-expand</button>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the scanner in expanded state with camera viewfinder active.',
      },
    },
  },
};

export const ErrorPermissionDenied: Story = {
  render: () => {
    const triggerError = () => {
      const scanner = document.querySelector('qr-scanner') as QrScanner | null;
      if (scanner) {
        // Inject mock provider that throws permission error
        scanner.setCameraProvider(
          createMockCameraProvider({
            getCamerasError: new CameraPermissionError(),
          })
        );
        scanner.expand();
      }
    };

    setTimeout(triggerError, 100);

    return html`
      <div style="max-width: 400px;">
        <h3>Permission Denied Error</h3>
        <p>Simulates what happens when camera permission is denied.</p>
        <qr-scanner
          @qr-scanned=${action('qr-scanned')}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>
        <button @click=${triggerError} style="margin-top: 10px;">Trigger Error</button>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when camera permission is denied by the user.',
      },
    },
  },
};

export const ErrorNoCamera: Story = {
  render: () => {
    const triggerError = () => {
      const scanner = document.querySelector('qr-scanner') as QrScanner | null;
      if (scanner) {
        // Inject mock provider that returns no cameras
        scanner.setCameraProvider(
          createMockCameraProvider({
            cameras: [], // Empty array triggers NoCameraError
          })
        );
        scanner.expand();
      }
    };

    setTimeout(triggerError, 100);

    return html`
      <div style="max-width: 400px;">
        <h3>No Camera Error</h3>
        <p>Simulates what happens when no camera is available.</p>
        <qr-scanner
          @qr-scanned=${action('qr-scanned')}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>
        <button @click=${triggerError} style="margin-top: 10px;">Trigger Error</button>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when no camera is detected on the device.',
      },
    },
  },
};

export const SimulatedScan: Story = {
  render: () => {
    const simulateScan = () => {
      const scanner = document.querySelector('qr-scanner') as QrScanner | null;
      if (scanner) {
        // Demo-only: access private _onScanSuccess to simulate a scan
        scanner.expand();
        setTimeout(() => {
          (scanner as unknown as { _onScanSuccess: (text: string) => void })._onScanSuccess('https://wiki.example.com/garage_toolbox/view');
        }, 1000);
      }
    };

    return html`
      <div style="max-width: 400px;">
        <h3>Simulated Scan</h3>
        <p>Click to simulate a successful QR code scan.</p>
        <qr-scanner
          @qr-scanned=${(e: CustomEvent) => {
            action('qr-scanned')(e);
            console.log('Scanned URL:', e.detail.rawValue);
          }}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>
        <button @click=${simulateScan} style="margin-top: 10px;">
          Simulate Scan
        </button>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console to see the scanned URL.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates a simulated successful scan. The scanner will auto-collapse after the scan.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const testUrls = [
      'https://wiki.example.com/garage_toolbox/view',
      'https://wiki.example.com/shelf_basement/view',
      '/drawer_kitchen/view',
      'storage_box_123',
    ];

    return html`
      <div style="max-width: 500px;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click "Scan QR Code" to open camera (requires permissions)</li>
          <li>Use "Simulate Scan" buttons to test different URL formats</li>
          <li>Check browser console for event details</li>
          <li>Click "Stop Scanning" or toggle button to close</li>
        </ul>

        <qr-scanner
          @qr-scanned=${(e: CustomEvent) => {
            action('qr-scanned')(e);
            console.log('Scanned:', e.detail.rawValue);
          }}
          @scanner-error=${action('scanner-error')}
        ></qr-scanner>

        <div style="margin-top: 15px;">
          <h4>Simulate Scans:</h4>
          <div style="display: flex; flex-direction: column; gap: 8px;">
            ${testUrls.map(url => html`
              <button @click=${() => {
                const scanner = document.querySelector('qr-scanner') as QrScanner | null;
                if (scanner) {
                  scanner.expand();
                  setTimeout(() => (scanner as unknown as { _onScanSuccess: (text: string) => void })._onScanSuccess(url), 500);
                }
              }} style="text-align: left; padding: 8px 12px;">
                <code>${url}</code>
              </button>
            `)}
          </div>
        </div>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>Toggle button opens/closes scanner</li>
            <li>Camera viewfinder appears when expanded</li>
            <li>Stop button closes scanner</li>
            <li>After successful scan, scanner auto-closes</li>
            <li>qr-scanned event fires with decoded URL</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing for the QR scanner component. Test different URL formats and observe events.',
      },
    },
  },
};
