/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './system-info-indexing.js';
import { SystemInfoIndexing } from './system-info-indexing.js';
import { GetJobStatusResponse, JobQueueStatus } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub } from 'sinon';

const meta: Meta = {
  title: 'Components/SystemInfoIndexing',
  component: 'system-info-indexing',
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component: 'A detailed job queue status component. Note: This is now a sub-component of SystemInfo. For the main UI component, see SystemInfo which combines version and job status in a compact overlay.'
      },
    },
  },
  argTypes: {},
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    
    // Set up realistic default data
    const frontmatterQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 50,
      highWaterMark: 500,
      isActive: true
    });

    const bleveQueue = new JobQueueStatus({
      name: 'Bleve',
      jobsRemaining: 120,
      highWaterMark: 500,
      isActive: true
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, bleveQueue]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default job queue status component with stubbed data showing job queue information.'
      },
    },
  },
};

export const Loading: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    el.loading = true;
    el.jobStatus = undefined;
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading state while fetching job queue status.'
      },
    },
  },
};

export const Idle: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: []
    });
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the idle state when no job queues are currently active.'
      },
    },
  },
};

export const ActiveIndexing: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
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
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, bleveQueue]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows active job queues with status for multiple queues. Demonstrates independent queue processing.'
      },
    },
  },
};

export const WithErrors: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const workingQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 50,
      highWaterMark: 500,
      isActive: true
    });

    const errorQueue = new JobQueueStatus({
      name: 'Embeddings',
      jobsRemaining: 375,
      highWaterMark: 500,
      isActive: false
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [workingQueue, errorQueue]
    });
    
    // For now, show errors through component error state until job queue error reporting is implemented
    el.error = 'Failed to connect to embedding service: timeout after 30s';
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows job queue status when queues are experiencing errors. Error states are displayed and error messages can be copied. Click on any error message to copy it to clipboard and see toast notifications. Open the browser developer tools console to see the action logs.'
      },
    },
  },
};

export const SlowIndexing: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const fastQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 500,
      highWaterMark: 10000,
      isActive: true
    });

    const slowQueue = new JobQueueStatus({
      name: 'AI-Embeddings',
      jobsRemaining: 7700,
      highWaterMark: 10000,
      isActive: true
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [fastQueue, slowQueue]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates the benefit of separate queues: fast job queues (Frontmatter) proceed independently while slow AI-powered queues (AI-Embeddings) run at their own pace.'
      },
    },
  },
};

export const Complete: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    const queue1 = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 0,
      highWaterMark: 1000,
      isActive: false
    });

    const queue2 = new JobQueueStatus({
      name: 'Bleve',
      jobsRemaining: 0,
      highWaterMark: 1000,
      isActive: false
    });

    const queue3 = new JobQueueStatus({
      name: 'Embeddings',
      jobsRemaining: 0,
      highWaterMark: 1000,
      isActive: false
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [queue1, queue2, queue3]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows completed job processing with all queues empty. Queue displays show completion status.'
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    el.loading = false;
    el.error = 'Failed to connect to indexing service';
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when the component cannot fetch job queue status.'
      },
    },
  },
};

export const MultipleIndexTypes: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls to prevent network requests
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    // Demonstrate different progress levels for each queue type
    const frontmatterQueue = new JobQueueStatus({
      name: 'Frontmatter',
      jobsRemaining: 200,
      highWaterMark: 3000,
      isActive: true
    });

    const bleveQueue = new JobQueueStatus({
      name: 'Bleve',
      jobsRemaining: 1050,
      highWaterMark: 3000,
      isActive: true
    });

    const embeddingsQueue = new JobQueueStatus({
      name: 'AI-Embeddings',
      jobsRemaining: 2550,
      highWaterMark: 3000,
      isActive: true
    });

    const vectorQueue = new JobQueueStatus({
      name: 'Vector-Search',
      jobsRemaining: 2250,
      highWaterMark: 3000,
      isActive: true
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [frontmatterQueue, bleveQueue, embeddingsQueue, vectorQueue]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows multiple queue types with different job levels, demonstrating how separate queues allow each queue to process independently. Notice how the fast Frontmatter queue has fewer jobs remaining while AI-powered queues are still processing many jobs.'
      },
    },
  },
};

export const InteractiveDemo: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // For interactive demo, we'll use a more realistic stubbed scenario
    // instead of real API calls to prevent 404 errors
    stub(el, 'loadStatus' as any).callsFake(async () => {
      // Simulate loading time
      await new Promise(resolve => setTimeout(resolve, 500));
      
      // Set up dynamic demo data
      const frontmatterQueue = new JobQueueStatus({
        name: 'Frontmatter',
        jobsRemaining: Math.floor(Math.random() * 100) + 50,
        highWaterMark: 500,
        isActive: Math.random() > 0.2
      });

      const bleveQueue = new JobQueueStatus({
        name: 'Bleve',
        jobsRemaining: Math.floor(Math.random() * 150) + 100,
        highWaterMark: 500,
        isActive: Math.random() > 0.2
      });

      el.jobStatus = new GetJobStatusResponse({
        jobQueues: [frontmatterQueue, bleveQueue]
      });
      
      el.loading = false;
      el.requestUpdate();
    });
    
    // Allow auto-refresh to work with stubbed data
    stub(el, 'startAutoRefresh' as any).callsFake(() => {
      // Call loadStatus periodically with stubbed data
      const interval = setInterval(() => {
        if (el.isConnected) {
          (el as any).loadStatus();
        } else {
          clearInterval(interval);
        }
      }, 3000); // Refresh every 3 seconds for demo
    });
    
    stub(el, 'stopAutoRefresh' as any);
    
    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Interactive Indexing Status Demo</h3>
        <p>This component demonstrates stubbed auto-refresh behavior with randomized job queue data.</p>
        <p><strong>Job Queue Status:</strong> Shows individual status for each job queue type.</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Demo refreshes every 3 seconds with simulated job queue updates. No real API calls are made.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive demo with stubbed auto-refresh behavior. Shows job queue status and simulates realistic queue processing scenarios without making API calls.'
      },
    },
  },
};

export const InteractiveErrorTesting: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    // Multiple error scenarios for comprehensive testing
    const networkQueue = new JobQueueStatus({
      name: 'Bleve-Search',
      jobsRemaining: 950,
      highWaterMark: 1000,
      isActive: false
    });

    const permissionQueue = new JobQueueStatus({
      name: 'File-Indexer',
      jobsRemaining: 800,
      highWaterMark: 1000,
      isActive: true
    });

    const validationQueue = new JobQueueStatus({
      name: 'AI-Embeddings',
      jobsRemaining: 925,
      highWaterMark: 1000,
      isActive: false
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [networkQueue, permissionQueue, validationQueue]
    });
    
    // For now, show errors through component error state until job queue error reporting is implemented
    el.error = 'Multiple queue errors: Network timeout, Permission denied, Validation failed';
    
    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Interactive Error Copy Testing</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click on any red error message to copy to clipboard</li>
          <li>Use Tab key to navigate to error messages, then press Enter/Space</li>
          <li>Watch for toast notifications confirming copy success</li>
          <li>Test with different error message lengths and content</li>
          <li>Try rapid clicking to test multiple copy operations</li>
        </ul>
        
        ${el}
        
        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Error text copied to system clipboard</li>
            <li>✅ Green success toast appears: "Error copied to clipboard"</li>
            <li>✅ Keyboard navigation works (Tab → Enter/Space)</li>
            <li>✅ Visual feedback on hover/focus</li>
            <li>⚠️ If clipboard fails: Red error toast appears</li>
          </ul>
        </div>
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing story for error click-to-copy functionality. Demonstrates multiple error types for job queues, keyboard navigation, accessibility features, and provides clear testing instructions. Open the browser developer tools console to see the action logs.'
      },
    },
  },
};

export const KeyboardNavigationTesting: Story = {
  render: () => {
    const el = document.createElement('system-info-indexing') as SystemInfoIndexing;
    
    // Stub API calls
    stub(el, 'loadStatus' as any).resolves();
    stub(el, 'startAutoRefresh' as any);
    stub(el, 'stopAutoRefresh' as any);
    
    // Setup with job queues for focused testing
    const errorQueue = new JobQueueStatus({
      name: 'Test-Index',
      jobsRemaining: 400,
      highWaterMark: 500,
      isActive: false
    });

    const workingQueue = new JobQueueStatus({
      name: 'Working-Index',
      jobsRemaining: 300,
      highWaterMark: 500,
      isActive: true
    });

    el.loading = false;
    el.jobStatus = new GetJobStatusResponse({
      jobQueues: [errorQueue, workingQueue]
    });
    
    // For now, show errors through component error state until job queue error reporting is implemented
    el.error = 'Test error message for keyboard navigation testing. This is a longer error message to test text selection and copying behavior.';
    
    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Keyboard Navigation Test</h3>
        <p><strong>Keyboard Testing Steps:</strong></p>
        <ol style="margin: 10px 0; padding-left: 20px;">
          <li>Click in this text area, then press Tab to navigate to the error</li>
          <li>Verify the error message receives focus (should show outline)</li>
          <li>Press Enter or Space to trigger copy</li>
          <li>Verify toast notification appears</li>
          <li>Press Tab again to ensure focus moves properly</li>
        </ol>
        
        <div style="margin: 15px 0; padding: 10px; border: 1px solid #ddd;">
          <input type="text" placeholder="Start tabbing from here..." style="width: 100%; padding: 8px;" />
        </div>
        
        ${el}
        
        <div style="margin: 15px 0; padding: 10px; border: 1px solid #ddd;">
          <button>Tab should reach here after error</button>
        </div>
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Tests keyboard navigation and accessibility for error click-to-copy functionality for job queue errors. Focuses on Tab navigation, Enter/Space activation, and proper focus management. Open the browser developer tools console to see the action logs.'
      },
    },
  },
};