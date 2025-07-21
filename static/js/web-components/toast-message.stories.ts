import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import './toast-message.js';

const meta: Meta = {
  title: 'Components/ToastMessage',
  component: 'toast-message',
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    message: { control: 'text' },
    type: { 
      control: { type: 'select' },
      options: ['success', 'error', 'warning', 'info']
    },
    visible: { control: 'boolean' },
    autoClose: { control: 'boolean' },
    timeout: { control: 'number' },
  },
};

export default meta;
type Story = StoryObj;

export const Success: Story = {
  args: {
    message: 'Operation completed successfully!',
    type: 'success',
    visible: true,
    autoClose: false,
  },
  render: (args) => html`
    <toast-message 
      .message=${args.message}
      .type=${args.type}
      .visible=${args.visible}
      .autoClose=${args.autoClose}
      .timeout=${args.timeout}>
    </toast-message>
  `,
};

export const Error: Story = {
  args: {
    message: 'An error occurred while processing your request.',
    type: 'error',
    visible: true,
    autoClose: false,
  },
  render: (args) => html`
    <toast-message 
      .message=${args.message}
      .type=${args.type}
      .visible=${args.visible}
      .autoClose=${args.autoClose}
      .timeout=${args.timeout}>
    </toast-message>
  `,
};

export const Warning: Story = {
  args: {
    message: 'Please review your changes before saving.',
    type: 'warning',
    visible: true,
    autoClose: false,
  },
  render: (args) => html`
    <toast-message 
      .message=${args.message}
      .type=${args.type}
      .visible=${args.visible}
      .autoClose=${args.autoClose}
      .timeout=${args.timeout}>
    </toast-message>
  `,
};

export const Info: Story = {
  args: {
    message: 'Use Ctrl+K to quickly search the wiki.',
    type: 'info',
    visible: true,
    autoClose: false,
  },
  render: (args) => html`
    <toast-message 
      .message=${args.message}
      .type=${args.type}
      .visible=${args.visible}
      .autoClose=${args.autoClose}
      .timeout=${args.timeout}>
    </toast-message>
  `,
};