import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './error-display.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/ErrorDisplay',
  component: 'error-display',
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    augmentedError: { control: 'object' },
  },
};

export default meta;
type Story = StoryObj;

export const BasicError: Story = {
  render: () => {
    const originalError = new Error('Something went wrong');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.WARNING,
      'warning'
    );
    return html`
      <error-display .augmentedError=${augmentedError}></error-display>
    `;
  },
};

export const WithDetails: Story = {
  render: () => {
    const originalError = new Error('Failed to save document');
    originalError.stack = `Error: PERMISSION_DENIED
    at FrontmatterService.replaceFrontmatter (/api/v1/frontmatter:45)
    at async save (frontmatter-editor:234)
    
Caused by: User does not have write permission for this page`;
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.PERMISSION,
      'permission',
      'saving document'
    );
    return html`
      <error-display .augmentedError=${augmentedError}></error-display>
    `;
  },
};

export const CustomIcon: Story = {
  render: () => {
    const originalError = new Error('Network connection failed');
    originalError.stack = 'Could not connect to server at localhost:8050. Please check your network connection and try again.';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NETWORK,
      'network',
      'connecting to server'
    );
    return html`
      <error-display .augmentedError=${augmentedError}></error-display>
    `;
  },
};

export const LongErrorMessage: Story = {
  render: () => {
    const originalError = new Error('This is a very long error message that demonstrates how the component handles text wrapping when the message exceeds the container width and needs to wrap to multiple lines');
    originalError.stack = `A very detailed stack trace that also demonstrates wrapping:

Error: ValidationError: The frontmatter validation failed with multiple issues
    at validateFrontmatter (/lib/validators/frontmatter.js:123:45)
    at processDocument (/lib/processors/document.js:67:12)
    at DocumentService.save (/services/document.js:89:23)
    at FrontmatterEditorDialog.handleSave (/components/frontmatter-editor.js:234:19)
    at HTMLButtonElement.click (/components/frontmatter-editor.js:456:7)

Validation Issues:
- Field 'title' is required but missing
- Field 'author' contains invalid characters: <>{}[]
- Field 'date' must be in YYYY-MM-DD format, got: "yesterday"
- Array field 'tags' cannot contain empty strings`;
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.VALIDATION,
      'validation',
      'validating frontmatter'
    );
    return html`
      <div style="max-width: 400px;">
        <error-display .augmentedError=${augmentedError}></error-display>
      </div>
    `;
  },
};

export const MultipleDifferentErrors: Story = {
  render: () => {
    const networkErrorOriginal = new Error('Connection timeout');
    networkErrorOriginal.stack = 'Failed to connect to server after 30 seconds';
    const networkError = new AugmentedError(
      networkErrorOriginal,
      ErrorKind.NETWORK,
      'network',
      'connecting to server'
    );
    
    const permissionErrorOriginal = new Error('Access denied');
    permissionErrorOriginal.stack = 'User does not have write permissions for this resource';
    const permissionError = new AugmentedError(
      permissionErrorOriginal,
      ErrorKind.PERMISSION,
      'permission',
      'accessing resource'
    );
    
    const validationErrorOriginal = new Error('Invalid input data');
    validationErrorOriginal.stack = 'Required field "title" is missing';
    const validationError = new AugmentedError(
      validationErrorOriginal,
      ErrorKind.VALIDATION,
      'validation',
      'validating form data'
    );
    
    const customIconErrorOriginal = new Error('Custom error with emoji');
    const customIconError = new AugmentedError(
      customIconErrorOriginal,
      ErrorKind.ERROR,
      '🎯',
      'performing custom operation'
    );
    
    return html`
      <div style="display: flex; flex-direction: column; gap: 20px; max-width: 500px;">
        <h3>Different Error Types</h3>
        
        <error-display .augmentedError=${networkError}></error-display>
        <error-display .augmentedError=${permissionError}></error-display>
        <error-display .augmentedError=${validationError}></error-display>
        <error-display .augmentedError=${customIconError}></error-display>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates various error types using both standard icons (network, permission, validation) and custom icons (🎯). Standard icons provide consistency while custom icons allow for specific use cases.',
      },
    },
  },
};

export const InteractiveExample: Story = {
  render: () => {
    const originalError = new Error('Interactive Error Display');
    originalError.stack = `This is an interactive example showing the expand/collapse functionality.

Try clicking the "Show details" button to expand this section, then click "Hide details" to collapse it again.

You can also use keyboard navigation:
- Tab to focus the button
- Enter or Space to activate it

The component supports:
✓ Smooth animations
✓ Keyboard accessibility 
✓ Screen reader compatibility
✓ High contrast mode
✓ Reduced motion preferences`;
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      '🎮',
      'testing component functionality'
    );
    
    return html`
      <div style="padding: 20px; background: #f0f8ff; border-radius: 8px;">
        <h3>Error Display Component Test</h3>
        <p>This example demonstrates the interactive features of the error display component.</p>
        
        <error-display .augmentedError=${augmentedError}></error-display>
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Try the expand/collapse functionality by clicking the "Show details" button.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive example for testing the expand/collapse functionality and accessibility features.',
      },
    },
  },
};

export const FrontmatterEditorContext: Story = {
  render: () => {
    const originalError = new Error('Failed to load frontmatter');
    originalError.stack = `ConnectError: NOT_FOUND
    at FrontmatterService.getFrontmatter (/api/v1/frontmatter:23)
    at FrontmatterEditorDialog.loadFrontmatter (frontmatter-editor:156)
    at FrontmatterEditorDialog.connectedCallback (frontmatter-editor:89)
    
Caused by: Page does not exist or has been moved`;
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NOT_FOUND,
      'not-found',
      'loading frontmatter'
    );
    
    return html`
      <div style="max-width: 600px; padding: 20px; border: 1px solid #ddd; border-radius: 8px; background: white;">
        <h3 style="margin-top: 0;">Frontmatter Editor - Error State</h3>
        <p>This shows how the error display component would appear in the frontmatter editor context:</p>
        
        <error-display .augmentedError=${augmentedError}></error-display>
        
        <div style="margin-top: 20px; display: flex; gap: 10px; justify-content: flex-end;">
          <button style="padding: 8px 16px; background: #f3f4f6; border: 1px solid #d1d5db; border-radius: 4px;">Cancel</button>
          <button style="padding: 8px 16px; background: #374151; color: white; border: 1px solid #374151; border-radius: 4px;">Save</button>
        </div>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows how the error display component appears in the context of the frontmatter editor dialog.',
      },
    },
  },
};

export const KernelPanicContext: Story = {
  render: () => {
    const originalError = new Error('Unhandled application error');
    originalError.stack = `TypeError: Cannot read property 'data' of undefined
    at WikiPage.render (wiki-page.js:45:12)
    at LitElement.update (lit-element.js:332:19)
    at LitElement.performUpdate (lit-element.js:298:16)
    at LitElement._$commitUpdate (lit-element.js:242:23)
    
Caused by: Malformed data structure received from server`;
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    
    return html`
      <div style="position: relative; height: 300px; background: #1f2937; color: white; padding: 20px; border-radius: 8px; font-family: monospace;">
        <h3 style="margin-top: 0; color: #ef4444;">Application Error</h3>
        <p style="margin-bottom: 20px;">This shows how the error display appears in a kernel panic context:</p>
        
        <div style="background: rgba(0,0,0,0.2); padding: 15px; border-radius: 4px;">
          <error-display .augmentedError=${augmentedError}></error-display>
        </div>
        
        <p style="margin-top: 20px; font-size: 0.9em; opacity: 0.8;">
          The application has encountered an unrecoverable error. Please refresh the page to restart.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows how the error display component appears in a kernel panic (application crash) context.',
      },
    },
  },
};

