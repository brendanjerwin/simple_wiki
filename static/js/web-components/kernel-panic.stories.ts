import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './kernel-panic.js';

const meta: Meta = {
  title: 'Components/KernelPanic',
  tags: ['autodocs'],
  component: 'kernel-panic',
  argTypes: {
    message: {
      control: 'text',
      description: 'The error message to display',
    },
    error: {
      control: 'object',
      description: 'The error object with stack trace',
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

export const WithStackTrace: Story = {
  args: {
    message: 'Unexpected runtime error',
    error: (() => {
      const error = new Error('Cannot read property "value" of undefined');
      error.stack = `Error: Cannot read property "value" of undefined
    at FrontmatterEditorDialog._handleSave (frontmatter-editor-dialog.ts:245:12)
    at FrontmatterEditorDialog.render (frontmatter-editor-dialog.ts:180:5)
    at LitElement.update (lit-element.js:456:8)
    at LitElement.performUpdate (lit-element.js:402:12)`;
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

export const MinimalMessage: Story = {
  args: {
    message: '',
    error: null,
  },
  render: (args) => html`
    <kernel-panic 
      .message="${args.message}"
      .error="${args.error}">
    </kernel-panic>
  `,
};

// Interactive test for refresh button
export const InteractiveRefreshButton: Story = {
  render: () => {
    // Custom action logger for this story
    const action = (name: string) => (event: Event) => {
      console.log(`🎬 Action: ${name}`, {
        type: event.type,
        target: event.target,
        detail: (event as CustomEvent).detail,
        timestamp: new Date().toISOString()
      });
    };

    return html`
      <div style="padding: 20px; background: #fff8dc; border: 1px solid #ddd; border-radius: 8px;">
        <h3 style="margin-top: 0;">Kernel Panic Interaction Test</h3>
        <p>Click the "Refresh Page" button to test the refresh functionality:</p>
        <kernel-panic 
          .message="Test panic for interaction testing"
          .error="${new Error('Simulated error with stack trace\n  at testFunction (test.js:1:1)\n  at main (app.js:5:5)')}"
          @click="${action('refresh-button-clicked')}">
        </kernel-panic>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Watch the browser console (F12) to see the refresh button click event.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'This story demonstrates the kernel panic refresh button interaction. Click the "Refresh Page" button to see the click event logged. Watch the browser console (F12) for event tracking.',
      },
    },
  },
};