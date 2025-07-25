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