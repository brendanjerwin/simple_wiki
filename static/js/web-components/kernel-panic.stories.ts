import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './kernel-panic.js';
import { ErrorKind, AugmentedError } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/KernelPanic',
  tags: ['autodocs'],
  component: 'kernel-panic',
  argTypes: {
    augmentedError: {
      control: false, // Disable control due to getter delegation causing deep clone issues in Storybook
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
    augmentedError: (() => {
      const originalError = new Error('Something went wrong with the application');
      originalError.stack = `Error: Something went wrong with the application
    at Application.init (app.js:42:15)
    at window.onload (index.js:10:5)`;
      return new AugmentedError(
        originalError,
        ErrorKind.ERROR,
        'error',
        'initializing application'
      );
    })(),
  },
  render: (args) => html`
    <kernel-panic .augmentedError="${args['augmentedError']}"></kernel-panic>
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

export const NetworkError: Story = {
  args: {
    augmentedError: (() => {
      const originalError = new Error('Failed to connect to server');
      originalError.stack = `Error: Failed to connect to server
    at fetch (network.js:15:10)
    at ApiClient.makeRequest (api.js:25:8)
    at UserService.loadProfile (user.js:40:12)`;
      return new AugmentedError(
        originalError,
        ErrorKind.NETWORK,
        'network',
        'loading user profile'
      );
    })(),
  },
  render: (args) => html`
    <kernel-panic .augmentedError="${args['augmentedError']}"></kernel-panic>
  `,
};

export const PermissionError: Story = {
  args: {
    augmentedError: (() => {
      const originalError = new Error('Access denied: insufficient permissions');
      originalError.stack = `Error: Access denied: insufficient permissions
    at SecurityService.checkPermissions (security.js:30:12)
    at DocumentService.save (document.js:55:8)
    at Editor.saveDocument (editor.js:120:15)`;
      return new AugmentedError(
        originalError,
        ErrorKind.PERMISSION,
        'permission',
        'saving document'
      );
    })(),
  },
  render: (args) => html`
    <kernel-panic .augmentedError="${args['augmentedError']}"></kernel-panic>
  `,
};

export const ValidationError: Story = {
  args: {
    augmentedError: (() => {
      const originalError = new Error('Invalid data format: missing required field "title"');
      originalError.stack = `Error: Invalid data format: missing required field "title"
    at Validator.validate (validator.js:18:9)
    at Form.submit (form.js:45:12)
    at HTMLFormElement.onsubmit (page.js:85:5)`;
      return new AugmentedError(
        originalError,
        ErrorKind.VALIDATION,
        'validation',
        'submitting form'
      );
    })(),
  },
  render: (args) => html`
    <kernel-panic .augmentedError="${args['augmentedError']}"></kernel-panic>
  `,
};

export const TimeoutError: Story = {
  args: {
    augmentedError: (() => {
      const originalError = new Error('Request timeout after 30 seconds');
      originalError.stack = `Error: Request timeout after 30 seconds
    at TimeoutHandler.onTimeout (timeout.js:22:8)
    at XMLHttpRequest.ontimeout (http.js:78:12)
    at BackupService.createBackup (backup.js:95:15)`;
      return new AugmentedError(
        originalError,
        ErrorKind.TIMEOUT,
        'timeout',
        'creating backup'
      );
    })(),
  },
  render: (args) => html`
    <kernel-panic .augmentedError="${args['augmentedError']}"></kernel-panic>
  `,
};
