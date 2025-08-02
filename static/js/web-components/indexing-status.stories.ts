/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './system-info-indexing.js';
import { SystemInfoIndexing } from './system-info-indexing.js';
import { GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';
import { stub } from 'sinon';

const meta: Meta = {
  title: 'Components/SystemInfoIndexing',
  component: 'system-info-indexing',
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component: 'A detailed indexing status component. Note: This is now a sub-component of SystemInfo. For the main UI component, see SystemInfo which combines version and indexing status in a compact overlay.',
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
    const frontmatterIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 450,
      total: 500,
      processingRatePerSecond: 25.3,
      lastError: undefined
    });

    const bleveIndex = new SingleIndexProgress({
      name: 'bleve',
      completed: 380,
      total: 500,
      processingRatePerSecond: 18.7,
      lastError: undefined
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 500,
      completedPages: 380, // Limited by slowest index
      queueDepth: 120,
      processingRatePerSecond: 22.0,
      indexProgress: [frontmatterIndex, bleveIndex]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default indexing status component with stubbed data showing per-index progress bars.',
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
    el.status = undefined;
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading state while fetching indexing status.',
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
    el.status = new GetIndexingStatusResponse({
      isRunning: false,
      totalPages: 0,
      completedPages: 0,
      queueDepth: 0,
      processingRatePerSecond: 0,
      indexProgress: []
    });
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the idle state when no indexing is currently running.',
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
    
    // Create mock timestamp 5 minutes from now
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor((Date.now() + 300000) / 1000)),
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
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 1000,
      completedPages: 720, // Minimum of the indexes
      queueDepth: 45,
      processingRatePerSecond: 28.9,
      estimatedCompletion: mockTimestamp,
      indexProgress: [frontmatterIndex, bleveIndex]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows active indexing with progress for multiple indexes. The overall progress is determined by the slowest index.',
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
    
    const workingIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 450,
      total: 500,
      processingRatePerSecond: 25.0,
      lastError: undefined
    });

    const errorIndex = new SingleIndexProgress({
      name: 'embeddings',
      completed: 125,
      total: 500,
      processingRatePerSecond: 2.1,
      lastError: 'Failed to connect to embedding service: timeout after 30s'
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 500,
      completedPages: 125, // Limited by the failing index
      queueDepth: 375,
      processingRatePerSecond: 13.6,
      indexProgress: [workingIndex, errorIndex]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows indexing status when one index is experiencing errors. The error index progress bar is styled differently and error messages are displayed. Click on any error message to copy it to clipboard and see toast notifications. Open the browser developer tools console to see the action logs.',
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
    
    // Create mock timestamp 2 hours from now
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor((Date.now() + 7200000) / 1000)),
      nanos: 0
    });

    const fastIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 9500,
      total: 10000,
      processingRatePerSecond: 150.5,
      lastError: undefined
    });

    const slowIndex = new SingleIndexProgress({
      name: 'ai-embeddings',
      completed: 2300,
      total: 10000,
      processingRatePerSecond: 0.8, // Very slow AI processing
      lastError: undefined
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 10000,
      completedPages: 2300, // Limited by slow AI index
      queueDepth: 7700,
      processingRatePerSecond: 75.6,
      estimatedCompletion: mockTimestamp,
      indexProgress: [fastIndex, slowIndex]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates the benefit of separate queues: fast indexing (frontmatter) proceeds independently while slow AI-powered indexing (embeddings) runs at its own pace.',
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
    
    const index1 = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 1000,
      total: 1000,
      processingRatePerSecond: 0,
      lastError: undefined
    });

    const index2 = new SingleIndexProgress({
      name: 'bleve',
      completed: 1000,
      total: 1000,
      processingRatePerSecond: 0,
      lastError: undefined
    });

    const index3 = new SingleIndexProgress({
      name: 'embeddings',
      completed: 1000,
      total: 1000,
      processingRatePerSecond: 0,
      lastError: undefined
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: false,
      totalPages: 1000,
      completedPages: 1000,
      queueDepth: 0,
      processingRatePerSecond: 0,
      indexProgress: [index1, index2, index3]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows completed indexing with all indexes at 100%. Progress bars are styled with completion colors.',
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
        story: 'Shows the error state when the component cannot fetch indexing status.',
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
    
    // Create mock timestamp 15 minutes from now
    const mockTimestamp = new Timestamp({
      seconds: BigInt(Math.floor((Date.now() + 900000) / 1000)),
      nanos: 0
    });

    // Demonstrate different progress levels for each index type
    const frontmatterIndex = new SingleIndexProgress({
      name: 'frontmatter',
      completed: 2800,
      total: 3000,
      processingRatePerSecond: 85.3,
      lastError: undefined
    });

    const bleveIndex = new SingleIndexProgress({
      name: 'bleve',
      completed: 1950,
      total: 3000,
      processingRatePerSecond: 22.1,
      lastError: undefined
    });

    const embeddingsIndex = new SingleIndexProgress({
      name: 'ai-embeddings',
      completed: 450,
      total: 3000,
      processingRatePerSecond: 1.2, // Very slow AI processing
      lastError: undefined
    });

    const vectorIndex = new SingleIndexProgress({
      name: 'vector-search',
      completed: 750,
      total: 3000,
      processingRatePerSecond: 3.8,
      lastError: undefined
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 3000,
      completedPages: 450, // Limited by slowest index (embeddings)
      queueDepth: 2550,
      processingRatePerSecond: 28.1,
      estimatedCompletion: mockTimestamp,
      indexProgress: [frontmatterIndex, bleveIndex, embeddingsIndex, vectorIndex]
    });
    
    return el;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows multiple index types with different progress levels, demonstrating how separate queues allow each index to progress independently. Notice how the fast frontmatter index is nearly complete while AI-powered indexes are still processing.',
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
      const frontmatterIndex = new SingleIndexProgress({
        name: 'frontmatter',
        completed: Math.floor(Math.random() * 100) + 400,
        total: 500,
        processingRatePerSecond: Math.random() * 50 + 20,
        lastError: undefined
      });

      const bleveIndex = new SingleIndexProgress({
        name: 'bleve',
        completed: Math.floor(Math.random() * 150) + 250,
        total: 500,
        processingRatePerSecond: Math.random() * 20 + 10,
        lastError: undefined
      });

      el.status = new GetIndexingStatusResponse({
        isRunning: Math.random() > 0.3,
        totalPages: 500,
        completedPages: Math.min(frontmatterIndex.completed, bleveIndex.completed),
        queueDepth: Math.floor(Math.random() * 100) + 50,
        processingRatePerSecond: Math.random() * 30 + 15,
        indexProgress: [frontmatterIndex, bleveIndex]
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
        <p>This component demonstrates stubbed auto-refresh behavior with randomized progress data.</p>
        <p><strong>Per-Index Progress:</strong> Click on "Per-Index Progress" to see individual progress bars for each index type.</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Demo refreshes every 3 seconds with simulated progress updates. No real API calls are made.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive demo with stubbed auto-refresh behavior. Shows per-index progress bars and simulates realistic indexing scenarios without making API calls.',
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
    const networkError = new SingleIndexProgress({
      name: 'bleve-search',
      completed: 50,
      total: 1000,
      processingRatePerSecond: 0,
      lastError: 'Network timeout: Unable to reach search index service at localhost:9200. Connection refused after 30 seconds.'
    });

    const permissionError = new SingleIndexProgress({
      name: 'file-indexer',
      completed: 200,
      total: 1000,
      processingRatePerSecond: 5.2,
      lastError: 'Permission denied: Cannot access /protected/documents/sensitive.pdf. Insufficient privileges for indexing operation.'
    });

    const validationError = new SingleIndexProgress({
      name: 'ai-embeddings',
      completed: 75,
      total: 1000,
      processingRatePerSecond: 1.1,
      lastError: 'Validation failed: Document contains invalid UTF-8 sequences at byte offset 1024. Unable to process for embedding generation.'
    });

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 1000,
      completedPages: 50,
      queueDepth: 950,
      processingRatePerSecond: 6.3,
      indexProgress: [networkError, permissionError, validationError]
    });
    
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
        story: 'Comprehensive interactive testing story for error click-to-copy functionality. Demonstrates multiple error types, keyboard navigation, accessibility features, and provides clear testing instructions. Open the browser developer tools console to see the action logs.',
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
    
    // Setup with single error for focused testing
    const errorIndex = new SingleIndexProgress({
      name: 'test-index',
      completed: 100,
      total: 500,
      processingRatePerSecond: 10.0,
      lastError: 'Test error message for keyboard navigation testing. This is a longer error message to test text selection and copying behavior.'
    });

    const workingIndex = new SingleIndexProgress({
      name: 'working-index',
      completed: 200,
      total: 500,
      processingRatePerSecond: 15.0,
      lastError: undefined
    });

    el.loading = false; 
    el.status = new GetIndexingStatusResponse({
      isRunning: true,
      totalPages: 500,
      completedPages: 100,
      queueDepth: 400,
      processingRatePerSecond: 12.5,
      indexProgress: [errorIndex, workingIndex]
    });
    
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
        story: 'Tests keyboard navigation and accessibility for error click-to-copy. Focuses on Tab navigation, Enter/Space activation, and proper focus management. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};