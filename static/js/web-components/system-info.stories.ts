/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './system-info.js';
import { SystemInfo } from './system-info.js';
import { GetVersionResponse, GetJobStatusResponse, JobQueueStatus } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub } from 'sinon';

const meta: Meta = {
  title: 'Components/SystemInfo',
  component: 'system-info',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: 'A compact system information overlay showing version info and job queue status. Positioned in the bottom-right corner, it remains mostly transparent until hovered.'
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

    const frontmatterQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 150,
      highWaterMark: 1000,
      isActive: true
    });

    const bleveQueue = new JobQueueStatus({
      name: 'Bleve',
      jobsRemaining: 280,
      highWaterMark: 1000,
      isActive: true
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'abc123def456',
      buildTime: mockTimestamp
    });
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, bleveQueue]
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
        story: 'Default system info component with stubbed data showing job queue status. Hover over the bottom-right component to see full details.'
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
    el.jobStatus = undefined;
    
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
        story: 'Loading state displayed while fetching version and job queue status from the server.'
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
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: []
    });
    
    return html`
      <div style="height: 100vh; background: #2d3748; position: relative;">
        <div style="padding: 20px; color: white;">
          <h1>Version Info Only</h1>
          <p>When no job queues are active, only version information is shown.</p>
          <p>The component remains compact and unobtrusive.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Compact display showing only version information when no job queues are active. The commit hash is truncated for space efficiency.'
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
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: []
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

    // Add job queues to show capability
    const frontmatterQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 300,
      highWaterMark: 1500,
      isActive: true
    });

    const bleveQueue = new JobQueueStatus({
      name: 'Bleve',
      jobsRemaining: 655,
      highWaterMark: 1500,
      isActive: true
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'abc123def456',
      buildTime: mockTimestamp
    });
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, bleveQueue]
    });
    
    return html`
      <div style="height: 100vh; background: #e2e8f0; position: relative;">
        <div style="padding: 20px;">
          <h1>Active Indexing</h1>
          <p>When job queues are active, additional status information is displayed below the version.</p>
          <p>Notice the animated status indicator and compact queue display.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Active job queue state with queue status display. The status indicator pulses to show activity.'
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

    // Show how different queue types process at different speeds
    const frontmatterQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 200,
      highWaterMark: 5000,
      isActive: true
    });

    const aiEmbeddingsQueue = new JobQueueStatus({
      name: 'AI-Embeddings',
      jobsRemaining: 4875,
      highWaterMark: 5000,
      isActive: true
    });

    el.loading = false;
    el.version = new GetVersionResponse({
      commit: 'v2.1.0-beta (def789a)',
      buildTime: mockTimestamp
    });
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, aiEmbeddingsQueue]
    });
    
    return html`
      <div style="height: 100vh; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); position: relative;">
        <div style="padding: 20px; color: white;">
          <h1>Slow AI Indexing</h1>
          <p>Demonstrates the component with queues having many remaining jobs (AI embeddings).</p>
          <p>Queue status shows large job backlogs appropriately.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the component handling queues with large job backlogs like AI embeddings, with appropriate display formatting.'
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
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [
        new JobQueueStatus({
          name: 'Final-Cleanup',
          jobsRemaining: 13,
          highWaterMark: 1000,
          isActive: true
        })
      ]
    });
    
    return html`
      <div style="height: 100vh; background: #f7fafc; position: relative;">
        <div style="padding: 20px;">
          <h1>Near Completion</h1>
          <p>Job processing nearing completion with few jobs remaining in queues.</p>
        </div>
        ${el}
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the component when job processing is nearly complete, with few jobs remaining.'
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
    el.jobStatus = undefined;
    
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
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: []
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
              <li><strong>Job Queue Status:</strong> Shows individual status for each job queue</li>
              <li><strong>Progressive Enhancement:</strong> Shows job queues only when active</li>
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

      // Create dynamic job queue data
      const frontmatterQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: Math.floor(Math.random() * 200) + 50,
        highWaterMark: 1000,
        isActive: Math.random() > 0.1
      });

      const bleveQueue = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: Math.floor(Math.random() * 300) + 100,
        highWaterMark: 1000,
        isActive: Math.random() > 0.2
      });

      const embeddingQueue = new JobQueueStatus({
        name: 'AI-Embeddings',
        jobsRemaining: Math.floor(Math.random() * 500) + 200,
        highWaterMark: 1000,
        isActive: Math.random() > 0.3
      });

      el.version = new GetVersionResponse({
        commit: 'interactive-demo-abc123',
        buildTime: mockTimestamp
      });
      
      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [frontmatterQueue, bleveQueue, embeddingQueue]
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
              <li><strong>Job Queue Status:</strong> The component shows status for each job queue (Frontmatter, Bleve, AI-Embeddings)</li>
              <li><strong>Auto-refresh:</strong> Demo refreshes every 4 seconds with dynamic progress updates</li>
              <li><strong>Error Simulation:</strong> Occasionally shows AI service errors</li>
              <li><strong>Queue Activity:</strong> Notice how different queues have different activity states</li>
            </ol>
          </div>

          <div style="margin: 20px 0; padding: 15px; background: #e8f4fd; border-radius: 8px;">
            <h3>Job Queue Features:</h3>
            <ul style="margin: 10px 0;">
              <li><strong>Individual Queue Status:</strong> Each job queue shows remaining jobs and activity status</li>
              <li><strong>Independent Processing:</strong> Fast queues don't wait for slow ones</li>
              <li><strong>Queue Isolation:</strong> Jobs in different queues are processed independently</li>
              <li><strong>Activity Indicators:</strong> Shows which queues are currently active</li>
              <li><strong>Compact Display:</strong> Shows "QueueName: N jobs" format</li>
            </ul>
          </div>

          <div style="margin: 20px 0; padding: 15px; background: #fff2e8; border-radius: 8px;">
            <h3>Development Notes:</h3>
            <p>This demo uses stubbed data to prevent API calls while showcasing realistic indexing behavior.</p>
            <p>The component demonstrates the architectural benefit of separate job queues for different types of work.</p>
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