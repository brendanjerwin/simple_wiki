import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './error-display.js';

const meta: Meta = {
  title: 'Components/ErrorDisplay',
  component: 'error-display',
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    message: { control: 'text' },
    details: { control: 'text' },
    icon: { control: 'text' },
  },
};

export default meta;
type Story = StoryObj;

export const BasicError: Story = {
  args: {
    message: 'Something went wrong',
  },
  render: (args) => html`
    <error-display 
      .message=${args.message}
      .details=${args.details}
      .icon=${args.icon}>
    </error-display>
  `,
};

export const WithDetails: Story = {
  args: {
    message: 'Failed to save document',
    details: `Error: PERMISSION_DENIED
    at FrontmatterService.replaceFrontmatter (/api/v1/frontmatter:45)
    at async save (frontmatter-editor:234)
    
Caused by: User does not have write permission for this page`,
  },
  render: (args) => html`
    <error-display 
      .message=${args.message}
      .details=${args.details}
      .icon=${args.icon}>
    </error-display>
  `,
};

export const CustomIcon: Story = {
  args: {
    message: 'Network connection failed',
    details: 'Could not connect to server at localhost:8050. Please check your network connection and try again.',
    icon: 'network', // Using standard icon
  },
  render: (args) => html`
    <error-display 
      .message=${args.message}
      .details=${args.details}
      .icon=${args.icon}>
    </error-display>
  `,
};

export const LongErrorMessage: Story = {
  args: {
    message: 'This is a very long error message that demonstrates how the component handles text wrapping when the message exceeds the container width and needs to wrap to multiple lines',
    details: `A very detailed stack trace that also demonstrates wrapping:

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
- Array field 'tags' cannot contain empty strings`,
  },
  render: (args) => html`
    <div style="max-width: 400px;">
      <error-display 
        .message=${args.message}
        .details=${args.details}
        .icon=${args.icon}>
      </error-display>
    </div>
  `,
};

export const MultipleDifferentErrors: Story = {
  render: () => html`
    <div style="display: flex; flex-direction: column; gap: 20px; max-width: 500px;">
      <h3>Different Error Types</h3>
      
      <error-display 
        message="Validation Error"
        details="The form contains invalid data that must be corrected before saving."
        icon="validation">
      </error-display>

      <error-display 
        message="Network Timeout"
        details="Request timed out after 30 seconds. The server may be experiencing high load."
        icon="timeout">
      </error-display>

      <error-display 
        message="Permission Denied"
        details="You do not have sufficient permissions to perform this action. Contact your administrator."
        icon="permission">
      </error-display>

      <error-display 
        message="File Not Found"
        icon="not-found">
      </error-display>

      <error-display 
        message="Custom Icon Example"
        details="This error uses a custom emoji icon instead of a standard one."
        icon="🎯">
      </error-display>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates various error types using both standard icons (validation, timeout, permission, not-found) and custom icons (🎯). Standard icons provide consistency while custom icons allow for specific use cases.',
      },
    },
  },
};

export const InteractiveExample: Story = {
  args: {
    message: 'Interactive Error Display',
    details: `This is an interactive example showing the expand/collapse functionality.

Try clicking the "Show details" button to expand this section, then click "Hide details" to collapse it again.

You can also use keyboard navigation:
- Tab to focus the button
- Enter or Space to activate it

The component supports:
✓ Smooth animations
✓ Keyboard accessibility 
✓ Screen reader compatibility
✓ High contrast mode
✓ Reduced motion preferences`,
    icon: '🎮',
  },
  render: (args) => html`
    <div style="padding: 20px; background: #f0f8ff; border-radius: 8px;">
      <h3>Error Display Component Test</h3>
      <p>This example demonstrates the interactive features of the error display component.</p>
      
      <error-display 
        .message=${args.message}
        .details=${args.details}
        .icon=${args.icon}>
      </error-display>
      
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Try the expand/collapse functionality by clicking the "Show details" button.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive example for testing the expand/collapse functionality and accessibility features.',
      },
    },
  },
};

