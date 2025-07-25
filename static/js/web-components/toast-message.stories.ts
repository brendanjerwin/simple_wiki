import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './toast-message.js';

// Custom action logger for Storybook
const action = (name: string) => (event: Event) => {
  console.log(`🎬 Action: ${name}`, {
    type: event.type,
    target: event.target,
    detail: (event as CustomEvent).detail,
    timestamp: new Date().toISOString()
  });
};

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
      .timeout=${args.timeout}
      @click=${action('toast-clicked')}
      @show=${action('toast-shown')}
      @hide=${action('toast-hidden')}>
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
      .timeout=${args.timeout}
      @click=${action('toast-clicked')}
      @show=${action('toast-shown')}
      @hide=${action('toast-hidden')}>
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
      .timeout=${args.timeout}
      @click=${action('toast-clicked')}
      @show=${action('toast-shown')}
      @hide=${action('toast-hidden')}>
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
      .timeout=${args.timeout}
      @click=${action('toast-clicked')}
      @show=${action('toast-shown')}
      @hide=${action('toast-hidden')}>
    </toast-message>
  `,
};

export const AutoClose: Story = {
  args: {
    message: 'This message will auto-hide after 3 seconds',
    type: 'success',
    visible: true,
    autoClose: true,
    timeout: 3000,
  },
  render: (args) => html`
    <div style="position: relative; height: 100px; display: flex; align-items: center; justify-content: center;">
      <toast-message 
        .message=${args.message}
        .type=${args.type}
        .visible=${args.visible}
        .autoClose=${args.autoClose}
        .timeout=${args.timeout}
        @click=${action('toast-clicked')}
        @show=${action('toast-shown')}
        @hide=${action('toast-hidden')}>
      </toast-message>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'This toast demonstrates auto-close behavior. Watch the browser console to see the hide event after 3 seconds.',
      },
    },
  },
};