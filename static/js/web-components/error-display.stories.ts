import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './error-display.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/ErrorDisplay',
  component: 'error-display',
  parameters: {
    layout: 'centered',
    docs: {
      description: {
        component: `
A compact error display component styled to match the system-info design language.

**Key Visual Features:**
- Dark grey background (#2d2d2d) matching system-info component
- Red error accents (#dc3545) for clear error indication
- Compact monospace typography for technical error details
- Expandable stack trace with subtle red background
- Maintains error semantics while using system-info styling
- No opacity effects since it's always embedded in containers

**Usage:**
\`\`\`typescript
import { ErrorDisplay } from './error-display.js';
<error-display .augmentedError=\${augmentedError}></error-display>
\`\`\`
        `,
      },
    },
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
