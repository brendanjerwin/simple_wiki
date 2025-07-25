import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './error-display.js';

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
    icon: '🚨',
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
        icon="❌">
      </error-display>

      <error-display 
        message="Network Timeout"
        details="Request timed out after 30 seconds. The server may be experiencing high load."
        icon="⏱️">
      </error-display>

      <error-display 
        message="Permission Denied"
        details="You do not have sufficient permissions to perform this action. Contact your administrator."
        icon="🔒">
      </error-display>

      <error-display 
        message="File Not Found"
        icon="📄">
      </error-display>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates various error types with different icons and detail levels.',
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
        .icon=${args.icon}
        @click=${action('error-display-clicked')}>
      </error-display>
      
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Open the browser developer tools console (F12) to see any logged events.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive example for testing the expand/collapse functionality and accessibility features. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const FrontmatterEditorContext: Story = {
  render: () => html`
    <div style="max-width: 600px; padding: 20px; border: 1px solid #ddd; border-radius: 8px; background: white;">
      <h3>Frontmatter Editor - Error State</h3>
      <p>This shows how the error display component would appear in the frontmatter editor context:</p>
      
      <error-display 
        message="Failed to load frontmatter"
        details="gRPC error: UNAVAILABLE - Connection to backend service failed. The server may be down or unreachable."
        icon="⚠️">
      </error-display>
      
      <div style="margin-top: 16px; display: flex; gap: 12px; justify-content: flex-end;">
        <button style="padding: 8px 16px; border: 1px solid #ccc; background: white; border-radius: 4px;">Cancel</button>
        <button style="padding: 8px 16px; border: 1px solid #ccc; background: #f8f9fa; border-radius: 4px;" disabled>Save</button>
      </div>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Shows how the error display component integrates into the frontmatter editor dialog context.',
      },
    },
  },
};

export const KernelPanicContext: Story = {
  render: () => html`
    <div style="background: #000; color: white; padding: 20px; border-radius: 8px; font-family: monospace;">
      <h3 style="color: white; margin-top: 0;">💀 Kernel Panic</h3>
      <p style="color: #ccc;">A critical error has occurred</p>
      
      <error-display 
        message="Unhandled exception in component lifecycle"
        details="TypeError: Cannot read property 'data' of undefined
    at FrontmatterEditor.render (frontmatter-editor.ts:234:56)
    at LitElement.performUpdate (lit-element.js:123:45)
    at LitElement.scheduleUpdate (lit-element.js:89:12)"
        icon="💥"
        style="background: #330000; border-color: #660000; color: #ffcccc;">
      </error-display>
      
      <button style="margin-top: 16px; padding: 12px 24px; background: #666; color: white; border: none; border-radius: 4px;">
        Refresh Page
      </button>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Shows how the error display component could integrate into the kernel panic component for better error presentation.',
      },
    },
  },
};