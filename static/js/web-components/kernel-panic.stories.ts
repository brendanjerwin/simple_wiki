import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './kernel-panic.js';
import type { AugmentedError } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/KernelPanic',
  tags: ['autodocs'],
  component: 'kernel-panic',
  argTypes: {
    processedError: {
      control: 'object',
      description: 'Augmented error object from AugmentErrorService',
    },
  },
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj;

export const Basic: Story = {
  args: {
    processedError: {
      message: 'Something went wrong with the application',
      details: `A critical error has occurred and the application cannot continue.

Technical details:
- Error type: System failure
- Component: Application core
- Timestamp: ${new Date().toISOString()}`,
      icon: 'error'
    } as AugmentedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};

export const WithoutError: Story = {
  render: () => html`
    <kernel-panic></kernel-panic>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Shows the kernel panic component without any error data. Only the basic instructions are shown.',
      },
    },
  },
};

export const WithDetailedError: Story = {
  args: {
    processedError: {
      message: 'Failed to save frontmatter data',
      details: `Network connection failed: Unable to reach server

Error: Network connection failed: Unable to reach server
    at WikiService.saveFrontmatter (wiki-service.ts:123:15)
    at FrontmatterEditorDialog._handleSave (frontmatter-editor-dialog.ts:245:12)
    at HTMLElement.click (frontmatter-editor-dialog.ts:180:5)

Connection details:
- URL: https://api.example.com/frontmatter
- Method: POST
- Status: Connection timeout
- Duration: 30 seconds`,
      icon: 'network'
    } as AugmentedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};

export const WithProcessedError: Story = {
  args: {
    processedError: {
      message: 'Critical system error',
      details: `Database connection failed: Unable to connect to PostgreSQL server

Connection timeout after 30 seconds
Server: localhost:5432
Database: wiki_production

Stack trace:
Error: connect ECONNREFUSED 127.0.0.1:5432
    at TCPConnectWrap.afterConnect [as oncomplete] (net.js:1141:16)
    at DatabaseService.connect (db-service.ts:45:12)
    at WikiService.initialize (wiki-service.ts:78:9)
    at Application.start (app.ts:123:15)`,
      icon: 'server'
    } as AugmentedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};

export const NetworkError: Story = {
  args: {
    processedError: {
      message: 'Unable to connect to server',
      details: `gRPC error: [unavailable] Failed to connect to backend

The server may be down or unreachable. Please check your network connection and try again.

Technical details:
- Connection timeout: 10 seconds
- Retry attempts: 3
- Last error: CONN_REFUSED`,
      icon: 'network'
    } as AugmentedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};

export const PermissionError: Story = {
  args: {
    processedError: {
      message: 'Access denied',
      details: `gRPC error: [permission_denied] Insufficient privileges

You do not have permission to perform this action.

Required permissions:
- system:admin
- wiki:critical_operations

Current user permissions:
- wiki:read
- wiki:write`,
      icon: 'permission'
    } as AugmentedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};