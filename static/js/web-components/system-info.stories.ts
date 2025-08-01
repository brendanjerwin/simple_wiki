import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './system-info.js';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';

const meta: Meta = {
  title: 'Components/SystemInfo',
  component: 'system-info',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: 'A compact system information overlay showing version info and indexing progress. Positioned in the bottom-right corner, it remains mostly transparent until hovered.',
      },
    },
  },
  argTypes: {},
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <div style="height: 100vh; background: #f0f8ff; position: relative;">
      <div style="padding: 20px;">
        <h1>System Info Demo</h1>
        <p>The system info component appears in the bottom-right corner. Hover over it to see full details.</p>
        <p>This component fetches real data from the server and auto-refreshes based on indexing status.</p>
      </div>
      <system-info></system-info>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Default system info component that fetches real data from the server. Auto-refreshes every 2 seconds when indexing is active, every 10 seconds when idle.',
      },
    },
  },
};

export const Loading: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    el.loading = true;
    el.version = undefined;
    el.indexingStatus = undefined;
    
    return html`
      <div style="height: 100vh; background: #f0f8ff; position: relative;">
        <div style="padding: 20px;">
          <h1>Loading State</h1>
          <p>Shows loading indicators while fetching system information.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Loading state displayed while fetching version and indexing status from the server.',
      },
    },
  },
};

export const VersionOnly: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'abc123def456789',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: false,
      totalPages: 0,
      completedPages: 0,
      queueDepth: 0,
      processingRatePerSecond: 0,
      indexProgress: []
    });
    
    return html`
      <div style="height: 100vh; background: #2d3748; position: relative;">
        <div style="padding: 20px; color: white;">
          <h1>Version Info Only</h1>
          <p>When indexing is idle, only version information is shown.</p>
          <p>The component remains compact and unobtrusive.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Compact display showing only version information when no indexing is active. The commit hash is truncated for space efficiency.',
      },
    },
  },
};

export const TaggedVersion: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'v1.2.3 (abc123d)',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: false,
      totalPages: 0,
      completedPages: 0,
      queueDepth: 0,
      processingRatePerSecond: 0,
      indexProgress: []
    });
    
    return html`
      <div style="height: 100vh; background: #1a202c; position: relative;">
        <div style="padding: 20px; color: white;">
          <h1>Tagged Version</h1>
          <p>Tagged versions (with parentheses) are displayed in full without truncation.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows how tagged versions are displayed without truncation, preserving the full version string.',
      },
    },
  },
};

export const ActiveIndexing: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'abc123def456',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 1500,
      completedPages: 845,
      queueDepth: 235,
      processingRatePerSecond: 28.5,
      indexProgress: []
    });
    
    return html`
      <div style="height: 100vh; background: #e2e8f0; position: relative;">
        <div style="padding: 20px;">
          <h1>Active Indexing</h1>
          <p>When indexing is running, additional progress information is displayed below the version.</p>
          <p>Notice the animated status indicator and compact progress bar.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Active indexing state with progress bar, processing rate, and queue depth. The status indicator pulses to show activity.',
      },
    },
  },
};

export const SlowIndexing: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'v2.1.0-beta (def789a)',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 5000,
      completedPages: 125,
      queueDepth: 4875,
      processingRatePerSecond: 0.7, // Very slow
      indexProgress: []
    });
    
    return html`
      <div style="height: 100vh; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); position: relative;">
        <div style="padding: 20px; color: white;">
          <h1>Slow AI Indexing</h1>
          <p>Demonstrates the component with very slow processing rates (AI embeddings).</p>
          <p>Processing rate is formatted appropriately for sub-1/second rates.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the component handling very slow indexing operations like AI embeddings, with appropriate rate formatting.',
      },
    },
  },
};

export const NearCompletion: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'main-branch-abc123d',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 1000,
      completedPages: 987,
      queueDepth: 13,
      processingRatePerSecond: 15.2,
      indexProgress: []
    });
    
    return html`
      <div style="height: 100vh; background: #f7fafc; position: relative;">
        <div style="padding: 20px;">
          <h1>Near Completion</h1>
          <p>Indexing process nearing completion with small queue depth remaining.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the component when indexing is nearly complete, with a small remaining queue.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    el.loading = false;
    el.error = 'Failed to connect to system info service';
    el.version = undefined;
    el.indexingStatus = undefined;
    
    return html`
      <div style="height: 100vh; background: #fed7d7; position: relative;">
        <div style="padding: 20px;">
          <h1>Error State</h1>
          <p>When the component cannot fetch system information, it displays an error message.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Error state displayed when the component cannot connect to the system info service.',
      },
    },
  },
};

export const ResponsiveDemo: Story = {
  render: () => html`
    <div style="height: 100vh; background: #e6fffa; position: relative;">
      <div style="padding: 20px;">
        <h1>Responsive Positioning</h1>
        <p>The system info component maintains its bottom-right position regardless of viewport size.</p>
        <p>Try resizing the browser window to see how it behaves.</p>
        <div style="margin: 20px 0; padding: 15px; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
          <h3>Component Features:</h3>
          <ul style="margin: 10px 0;">
            <li><strong>Fixed Positioning:</strong> Always visible in bottom-right corner</li>
            <li><strong>Low Opacity:</strong> Unobtrusive until hovered</li>
            <li><strong>Compact Display:</strong> Minimal space usage</li>
            <li><strong>Auto-refresh:</strong> Updates based on indexing status</li>
            <li><strong>Progressive Enhancement:</strong> Shows indexing only when active</li>
          </ul>
        </div>
      </div>
      <system-info></system-info>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates the responsive behavior and positioning of the system info component. The component maintains its position and functionality across different viewport sizes.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => html`
    <div style="height: 100vh; background: #f0f8ff; position: relative; overflow: auto;">
      <div style="padding: 20px; max-width: 800px;">
        <h1>Interactive System Info Testing</h1>
        <p>This story demonstrates the complete behavior of the system-info component.</p>
        
        <div style="margin: 20px 0; padding: 15px; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
          <h3>Testing Instructions:</h3>
          <ol style="margin: 10px 0; padding-left: 20px;">
            <li><strong>Hover Interaction:</strong> Hover over the bottom-right component to see it become more visible</li>
            <li><strong>Auto-refresh:</strong> The component automatically refreshes based on indexing status</li>
            <li><strong>Version Display:</strong> Always shows commit hash and build time</li>
            <li><strong>Indexing Progress:</strong> Shows progress bar and stats only when indexing is active</li>
            <li><strong>Error Handling:</strong> Displays appropriate error messages if API calls fail</li>
          </ol>
        </div>

        <div style="margin: 20px 0; padding: 15px; background: #e8f4fd; border-radius: 8px;">
          <h3>Expected Behavior:</h3>
          <ul style="margin: 10px 0;">
            <li>Component appears in bottom-right corner with low opacity</li>
            <li>On hover, opacity increases to show full details</li>
            <li>Version information is always visible</li>
            <li>Indexing section only appears when <code>isRunning: true</code></li>
            <li>Progress bar reflects completion percentage</li>
            <li>Processing rate is formatted appropriately (e.g., "< 0.1/s", "12/s")</li>
            <li>Queue depth shown only when > 0</li>
          </ul>
        </div>

        <div style="margin: 20px 0; padding: 15px; background: #fff2e8; border-radius: 8px;">
          <h3>Development Notes:</h3>
          <p>Open the browser developer tools console to see any network requests and component behavior logs.</p>
          <p>The component uses real API endpoints and will show actual system status when connected to a running server.</p>
        </div>
      </div>
      <system-info></system-info>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive testing environment for the system-info component. Use this to test hover behavior, auto-refresh functionality, and real API integration. Open the browser developer tools console to see network requests.',
      },
    },
  },
};