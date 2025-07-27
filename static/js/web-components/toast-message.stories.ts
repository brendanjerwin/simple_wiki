import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import { AugmentedError, ErrorKind } from './augment-error-service.js';
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
      .message=${args['message']}
      .type=${args['type']}
      .visible=${args['visible']}
      .autoClose=${args['autoClose']}
      .timeout=${args['timeout']}
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
      .message=${args['message']}
      .type=${args['type']}
      .visible=${args['visible']}
      .autoClose=${args['autoClose']}
      .timeout=${args['timeout']}
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
      .message=${args['message']}
      .type=${args['type']}
      .visible=${args['visible']}
      .autoClose=${args['autoClose']}
      .timeout=${args['timeout']}
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
      .message=${args['message']}
      .type=${args['type']}
      .visible=${args['visible']}
      .autoClose=${args['autoClose']}
      .timeout=${args['timeout']}
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
        .message=${args['message']}
        .type=${args['type']}
        .visible=${args['visible']}
        .autoClose=${args['autoClose']}
        .timeout=${args['timeout']}
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

export const AugmentedErrorDisplay: Story = {
  args: {
    type: 'error',
    visible: true,
    autoClose: false,
  },
  render: (args) => {
    // Create a sample AugmentedError for demonstration
    const originalError = new Error('Failed to connect to the server. The request timed out after 30 seconds.');
    originalError.stack = `Error: Failed to connect to the server. The request timed out after 30 seconds.
    at fetchData (http://localhost:8050/static/js/api.js:45:12)
    at async loadUserProfile (http://localhost:8050/static/js/components/user-profile.js:23:18)
    at async UserProfileComponent.connectedCallback (http://localhost:8050/static/js/components/user-profile.js:15:5)`;
    
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NETWORK,
      'network',
      'loading user profile'
    );

    return html`
      <toast-message 
        .type=${args['type']}
        .visible=${args['visible']}
        .autoClose=${args['autoClose']}
        .augmentedError=${augmentedError}
        @click=${action('toast-clicked')}
        @show=${action('toast-shown')}
        @hide=${action('toast-hidden')}>
      </toast-message>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'This toast demonstrates the AugmentedError functionality, embedding an error-display component with detailed error information including an expandable stack trace. Note how the error type disables auto-close by default.',
      },
    },
  },
};
