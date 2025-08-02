/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './system-info.js';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub } from 'sinon';

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
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    // Stub API calls to prevent 404 errors
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    // Set up default demo data
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    const frontmatterIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 850,
      total: 1000,
      processingRatePerSecond: 45.2,
      lastError: undefined
    });

    const bleveIndex = new SingleIndexProgress({
      name: 'bleve',
      completed: 720,
      total: 1000,
      processingRatePerSecond: 12.8,
      lastError: undefined
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'abc123def456',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 1000,
      completedPages: 720,
      queueDepth: 280,
      processingRatePerSecond: 29.0,
      indexProgress: [frontmatterIndex, bleveIndex]
    });
    
    return html`
      <div style="height: 100vh; background: #f0f8ff; position: relative;">
        <div style="padding: 20px;">
          <h1>System Info Demo</h1>
          <p>The system info component appears in the bottom-right corner. Hover over it to see full details.</p>
          <p>This component shows stubbed data and demonstrates per-index progress capabilities.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default system info component with stubbed data showing per-index progress. Hover over the bottom-right component to see full details.',
      },
    },
  },
};

export const Loading: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
    
    // Stub API calls to prevent 404 errors in Storybook
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
    
    // Stub API calls to prevent 404 errors in Storybook
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
    
    // Stub API calls to prevent network requests
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    // Add per-index progress to show capability
    const frontmatterIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 1200,
      total: 1500,
      processingRatePerSecond: 42.1,
      lastError: undefined
    });

    const bleveIndex = new SingleIndexProgress({
      name: 'bleve',
      completed: 845,
      total: 1500,
      processingRatePerSecond: 15.4,
      lastError: undefined
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
      indexProgress: [frontmatterIndex, bleveIndex]
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
    
    // Stub API calls to prevent network requests
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    // Show how different index types process at different speeds
    const frontmatterIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 4800,
      total: 5000,
      processingRatePerSecond: 25.3,
      lastError: undefined
    });

    const aiEmbeddingsIndex = new SingleIndexProgress({
      name: 'ai-embeddings',
      completed: 125,
      total: 5000,
      processingRatePerSecond: 0.7, // Very slow AI processing
      lastError: undefined
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'v2.1.0-beta (def789a)',
      buildTime: mockTimestamp
    });
    el.indexingStatus = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 5000,
      completedPages: 125, // Limited by slowest index
      queueDepth: 4875,
      processingRatePerSecond: 13.0,
      indexProgress: [frontmatterIndex, aiEmbeddingsIndex]
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
    
    // Stub API calls to prevent 404 errors in Storybook
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
    
    // Stub API calls to prevent network requests
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    // Stub API calls and set up demo data
    stub(el, 'loadSystemInfo' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
      nanos: 0
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'responsive-demo-123',
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
              <li><strong>Per-Index Progress:</strong> Shows individual progress for each index type</li>
              <li><strong>Progressive Enhancement:</strong> Shows indexing only when active</li>
            </ul>
          </div>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates the responsive behavior and positioning of the system info component. The component maintains its position and functionality across different viewport sizes.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const el = document.createElement('system-info') as SystemInfo;
    
    // Stub API calls with dynamic demo data
    stub(el, 'loadSystemInfo' as any).callsFake(async () => {
      // Simulate loading time
      await new Promise(resolve => setTimeout(resolve, 300));
      
      const mockTimestamp = new Timestamp({
        seconds: BigInt(Math.floor(new Date('2023-12-15T14:30:00Z').getTime() / 1000)),
        nanos: 0
      });

      // Create dynamic progress data
      const frontmatterProgress = Math.floor(Math.random() * 200) + 800;
      const bleveProgress = Math.floor(Math.random() * 300) + 500;
      const embeddingProgress = Math.floor(Math.random() * 100) + 50;
      
      const frontmatterIndex = new SingleIndexProgress({
        name: 'frontmatter',
        completed: frontmatterProgress,
        total: 1000,
        processingRatePerSecond: Math.random() * 20 + 30,
        lastError: undefined
      });

      const bleveIndex = new SingleIndexProgress({
        name: 'bleve',
        completed: bleveProgress,
        total: 1000,
        processingRatePerSecond: Math.random() * 10 + 15,
        lastError: undefined
      });

      const embeddingIndex = new SingleIndexProgress({
        name: 'ai-embeddings',
        completed: embeddingProgress,
        total: 1000,
        processingRatePerSecond: Math.random() * 2 + 0.5,
        lastError: Math.random() > 0.8 ? 'Temporary AI service timeout' : undefined
      });

      el.version = new GetVersionResponse({
        commit: 'interactive-demo-abc123',
        buildTime: mockTimestamp
      });
      
      const isRunning = Math.random() > 0.2;
      const minProgress = Math.min(frontmatterProgress, bleveProgress, embeddingProgress);
      
      el.indexingStatus = new GetIndexingStatusResponse({
        isRunning,
        totalPages: 1000,
        completedPages: minProgress,
        queueDepth: isRunning ? Math.floor(Math.random() * 200) + 50 : 0,
        processingRatePerSecond: isRunning ? Math.random() * 15 + 20 : 0,
        indexProgress: [frontmatterIndex, bleveIndex, embeddingIndex]
      });
      
      el.loading = false;
      el.requestUpdate();
    });
    
    // Enable auto-refresh with stubbed data
    stub(el, 'startAutoRefresh' as any).callsFake(() => {
      const interval = setInterval(() => {
        if (el.isConnected) {
          (el as any).loadSystemInfo();
        } else {
          clearInterval(interval);
        }
      }, 4000); // Refresh every 4 seconds for demo
    });
    
    stub(el, 'stopAutoRefresh' as any);
    
    return html`
      <div style="height: 100vh; background: #f0f8ff; position: relative; overflow: auto;">
        <div style="padding: 20px; max-width: 800px;">
          <h1>Interactive System Info Testing</h1>
          <p>This story demonstrates the complete behavior of the system-info component with per-index progress.</p>
          
          <div style="margin: 20px 0; padding: 15px; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
            <h3>Testing Instructions:</h3>
            <ol style="margin: 10px 0; padding-left: 20px;">
              <li><strong>Hover Interaction:</strong> Hover over the bottom-right component to see it become more visible</li>
              <li><strong>Per-Index Progress:</strong> The component shows progress for each index type (frontmatter, bleve, ai-embeddings)</li>
              <li><strong>Auto-refresh:</strong> Demo refreshes every 4 seconds with dynamic progress updates</li>
              <li><strong>Error Simulation:</strong> Occasionally shows AI service errors</li>
              <li><strong>Rate Differences:</strong> Notice how different indexes process at different speeds</li>
            </ol>
          </div>

          <div style="margin: 20px 0; padding: 15px; background: #e8f4fd; border-radius: 8px;">
            <h3>Per-Index Progress Features:</h3>
            <ul style="margin: 10px 0;">
              <li><strong>Individual Progress Bars:</strong> Each index type has its own progress bar</li>
              <li><strong>Separate Processing Rates:</strong> Fast frontmatter, medium bleve, slow AI embeddings</li>
              <li><strong>Independent Queues:</strong> Fast indexes don't wait for slow ones</li>
              <li><strong>Error Handling:</strong> Individual indexes can fail without affecting others</li>
              <li><strong>Overall Progress:</strong> Limited by the slowest index (realistic behavior)</li>
            </ul>
          </div>

          <div style="margin: 20px 0; padding: 15px; background: #fff2e8; border-radius: 8px;">
            <h3>Development Notes:</h3>
            <p>This demo uses stubbed data to prevent API calls while showcasing realistic indexing behavior.</p>
            <p>The component demonstrates the architectural benefit of separate per-index worker queues.</p>
          </div>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive testing environment for the system-info component. Use this to test hover behavior, auto-refresh functionality, and real API integration. Open the browser developer tools console to see network requests.',
      },
    },
  },
};