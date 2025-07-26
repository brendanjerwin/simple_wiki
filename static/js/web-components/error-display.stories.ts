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
    const augmentedError = new AugmentedError(
      'Something went wrong',
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
    const augmentedError = new AugmentedError(
      'Failed to save document',
      ErrorKind.PERMISSION,
      'permission',
      `Error: PERMISSION_DENIED
    at FrontmatterService.replaceFrontmatter (/api/v1/frontmatter:45)
    at async save (frontmatter-editor:234)
    
Caused by: User does not have write permission for this page`
    );
    return html`
      <error-display .augmentedError=${augmentedError}></error-display>
    `;
  },
};

export const CustomIcon: Story = {
  render: () => {
    const augmentedError = new AugmentedError(
      'Network connection failed',
      ErrorKind.NETWORK,
      'network',
      'Could not connect to server at localhost:8050. Please check your network connection and try again.'
    );
    return html`
      <error-display .augmentedError=${augmentedError}></error-display>
    `;
  },
};

export const LongErrorMessage: Story = {
  render: () => {
    const augmentedError = new AugmentedError(
      'This is a very long error message that demonstrates how the component handles text wrapping when the message exceeds the container width and needs to wrap to multiple lines',
      ErrorKind.VALIDATION,
      'validation',
      `A very detailed stack trace that also demonstrates wrapping:

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
- Array field 'tags' cannot contain empty strings`
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
    const networkError = new AugmentedError(
      'Connection timeout',
      ErrorKind.NETWORK,
      'network',
      'Failed to connect to server after 30 seconds'
    );
    
    const permissionError = new AugmentedError(
      'Access denied',
      ErrorKind.PERMISSION,
      'permission',
      'User does not have write permissions for this resource'
    );
    
    const validationError = new AugmentedError(
      'Invalid input data',
      ErrorKind.VALIDATION,
      'validation',
      'Required field "title" is missing'
    );
    
    const customIconError = new AugmentedError(
      'Custom error with emoji',
      ErrorKind.ERROR,
      '🎯'
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
    const augmentedError = new AugmentedError(
      'Interactive Error Display',
      ErrorKind.ERROR,
      '🎮',
      `This is an interactive example showing the expand/collapse functionality.

Try clicking the "Show details" button to expand this section, then click "Hide details" to collapse it again.

You can also use keyboard navigation:
- Tab to focus the button
- Enter or Space to activate it

The component supports:
✓ Smooth animations
✓ Keyboard accessibility 
✓ Screen reader compatibility
✓ High contrast mode
✓ Reduced motion preferences`
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

