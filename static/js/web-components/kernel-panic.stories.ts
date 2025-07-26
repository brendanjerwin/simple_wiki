import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './kernel-panic.js';
import type { ProcessedError } from './error-service.js';

const meta: Meta = {
  title: 'Components/KernelPanic',
  tags: ['autodocs'],
  component: 'kernel-panic',
  argTypes: {
    message: {
      control: 'text',
      description: 'The error message to display (legacy)',
    },
    error: {
      control: 'object',
      description: 'The error object with stack trace (legacy)',
    },
    processedError: {
      control: 'object',
      description: 'Processed error object from ErrorService',
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
    message: 'Something went wrong with the application',
    error: null,
  },
  render: (args) => html`
    <kernel-panic 
      .message="${args.message}"
      .error="${args.error}">
    </kernel-panic>
  `,
};

export const WithError: Story = {
  args: {
    message: 'Failed to save frontmatter data',
    error: (() => {
      const error = new Error('Network connection failed: Unable to reach server');
      error.stack = `Error: Network connection failed: Unable to reach server
    at WikiService.saveFrontmatter (wiki-service.ts:123:15)
    at FrontmatterEditorDialog._handleSave (frontmatter-editor-dialog.ts:245:12)
    at HTMLElement.click (frontmatter-editor-dialog.ts:180:5)`;
      return error;
    })(),
  },
  render: (args) => html`
    <kernel-panic 
      .message="${args.message}"
      .error="${args.error}">
    </kernel-panic>
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
    } as ProcessedError,
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
    } as ProcessedError,
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
    } as ProcessedError,
  },
  render: (args) => html`
    <kernel-panic .processedError="${args.processedError}"></kernel-panic>
  `,
};