import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from '@storybook/addon-actions';
import './indexing-status.js';
import { IndexingStatus } from './indexing-status.js';
import { GetIndexingStatusResponse, SingleIndexProgress } from '../gen/api/v1/system_info_pb.js';
import { Timestamp } from '@bufbuild/protobuf';

const meta: Meta = {
  title: 'Components/IndexingStatus',
  component: 'indexing-status',
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
  render: () => html`
    <indexing-status></indexing-status>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Default indexing status component that will fetch real data from the server.',
      },
    },
  },
};

export const Loading: Story = {
  render: () => {
    const el = document.createElement('indexing-status') as IndexingStatus;
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
    const el = document.createElement('indexing-status') as IndexingStatus;
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
    const el = document.createElement('indexing-status') as IndexingStatus;
    
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
    const el = document.createElement('indexing-status') as IndexingStatus;
    
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
        story: 'Shows indexing status when one index is experiencing errors. The error index progress bar is styled differently and error messages are displayed.',
      },
    },
  },
};

export const SlowIndexing: Story = {
  render: () => {
    const el = document.createElement('indexing-status') as IndexingStatus;
    
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
    const el = document.createElement('indexing-status') as IndexingStatus;
    
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

    el.loading = false;
    el.status = new GetIndexingStatusResponse({
      isRunning: false,
      totalPages: 1000,
      completedPages: 1000,
      queueDepth: 0,
      processingRatePerSecond: 0,
      indexProgress: [index1, index2]
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
    const el = document.createElement('indexing-status') as IndexingStatus;
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

export const InteractiveDemo: Story = {
  render: () => html`
    <div style="padding: 20px; background: #f0f8ff;">
      <h3>Indexing Status Component Demo</h3>
      <p>This component automatically refreshes every 2 seconds when indexing is active, and every 10 seconds when idle.</p>
      <indexing-status></indexing-status>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        The component will fetch real data from the server. Open the browser developer tools console to see any network requests.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive demo that connects to the real indexing status API. Use this to test the component with live data.',
      },
    },
  },
};