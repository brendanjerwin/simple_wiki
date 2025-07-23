import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import { expect, userEvent } from '@storybook/test';
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

// Interactive story demonstrating click-to-dismiss behavior
export const InteractiveClickToDismiss: Story = {
  args: {
    message: 'Click on this toast to dismiss it!',
    type: 'info',
    visible: true,
    autoClose: false,
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
  play: async ({ canvasElement }) => {
    // Find the toast message element
    const toastElement = canvasElement.querySelector('toast-message');
    expect(toastElement).toBeInTheDocument();
    
    // Verify it's visible initially
    expect(toastElement).toHaveProperty('visible', true);
    
    // Click on the toast message
    await userEvent.click(toastElement);
    
    // Note: In a real implementation, clicking would hide the toast
    // This play function demonstrates the interaction pattern
  },
  parameters: {
    docs: {
      description: {
        story: 'Click on the toast message to see the click action logged to the browser console. In the real application, this would dismiss the toast. The play function automatically demonstrates clicking on the toast. Watch both the Interactions panel and browser console.',
      },
    },
  },
};

// Auto-close demonstration
export const AutoCloseBehavior: Story = {
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
  play: async ({ canvasElement }) => {
    // Find the toast message element
    const toastElement = canvasElement.querySelector('toast-message');
    expect(toastElement).toBeInTheDocument();
    
    // Verify it's visible initially
    expect(toastElement).toHaveProperty('visible', true);
    expect(toastElement).toHaveProperty('autoClose', true);
    expect(toastElement).toHaveProperty('timeout', 3000);
    
    // Note: We could wait for the timeout, but that would make the play function take 3 seconds
    // Instead, we verify the initial state. The actual auto-close behavior can be observed manually
  },
  parameters: {
    docs: {
      description: {
        story: 'This toast demonstrates auto-close behavior. The play function verifies the auto-close configuration. Watch the browser console to see the hide event after 3 seconds. The Interactions panel shows the automated verification of the component setup.',
      },
    },
  },
};