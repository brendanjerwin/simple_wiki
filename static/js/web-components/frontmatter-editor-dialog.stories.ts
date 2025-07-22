import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { expect, userEvent, within } from '@storybook/test';
import './frontmatter-editor-dialog.js';

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
  title: 'Components/FrontmatterEditorDialog',
  tags: ['autodocs'],
  component: 'frontmatter-editor-dialog',
  argTypes: {
    page: {
      control: 'text',
      description: 'The page identifier',
    },
    open: {
      control: 'boolean',
      description: 'Whether the dialog is open',
    },
  },
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj;

// Note: These stories show the visual states of the dialog component.
// In Storybook, we can manually set the component's internal state properties
// to demonstrate different UI states without requiring a gRPC backend.

export const Closed: Story = {
  args: {
    page: 'sample-page',
    open: false,
  },
  render: (args) => html`
    <frontmatter-editor-dialog 
      .page="${args.page}"
      .open="${args.open}"
      @save=${action('save-event')}
      @cancel=${action('cancel-event')}
      @close=${action('close-event')}>
    </frontmatter-editor-dialog>
  `,
};

export const LoadingState: Story = {
  args: {
    page: 'loading-page',
    open: true,
  },
  render: (args) => {
    // Create element and manually set its internal loading state
    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${true}"
        @save=${action('save-event')}
        @cancel=${action('cancel-event')}
        @close=${action('close-event')}>
    </frontmatter-editor-dialog>
    `;
  },
};

export const ErrorState: Story = {
  args: {
    page: 'error-page',
    open: true,
  },
  render: (args) => html`
    <frontmatter-editor-dialog 
      .page="${args.page}"
      .open="${args.open}"
      .loading="${false}"
      .error="${'Network error: Could not connect to server'}"
      @save=${action('save-event')}
      @cancel=${action('cancel-event')}
      @close=${action('close-event')}>
    </frontmatter-editor-dialog>
  `,
};

export const WithFrontmatterData: Story = {
  args: {
    page: 'content-page',
    open: true,
  },
  render: (args) => {
    // Simulate component with loaded frontmatter data
    const mockFrontmatterData = {
      title: 'Sample Page Title',
      author: 'John Doe',
      date: '2024-01-15',
      tags: ['documentation', 'example', 'storybook'],
      category: 'tutorial',
      published: true,
      metadata: {
        version: '1.0',
        lastModified: '2024-01-15T10:30:00Z'
      }
    };

    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${false}"
        .workingFrontmatter="${mockFrontmatterData}"
        @save=${action('save-event')}
        @cancel=${action('cancel-event')}
        @close=${action('close-event')}
        @value-change=${action('value-changed')}
        @key-change=${action('key-changed')}
        @add-field=${action('field-added')}>
      </frontmatter-editor-dialog>
    `;
  },
};

export const SavingState: Story = {
  args: {
    page: 'saving-page',
    open: true,
  },
  render: (args) => {
    const mockFrontmatterData = {
      title: 'Page Being Saved',
      author: 'Jane Smith',
      status: 'draft'
    };

    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${false}"
        .saving="${true}"
        .workingFrontmatter="${mockFrontmatterData}"
        @save=${action('save-event')}
        @cancel=${action('cancel-event')}
        @close=${action('close-event')}
        @value-change=${action('value-changed')}
        @key-change=${action('key-changed')}
        @add-field=${action('field-added')}>
      </frontmatter-editor-dialog>
    `;
  },
};

// Interactive form testing story
export const InteractiveFormTesting: Story = {
  args: {
    page: 'interactive-test',
    open: true,
  },
  render: (args) => {
    const mockFrontmatterData = {
      title: 'Edit Me!',
      description: 'Try editing these fields',
      tags: ['test', 'interactive'],
      published: false,
    };

    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${false}"
        .workingFrontmatter="${mockFrontmatterData}"
        @save=${action('save-event')}
        @cancel=${action('cancel-event')}
        @close=${action('close-event')}
        @value-change=${action('value-changed')}
        @key-change=${action('key-changed')}
        @add-field=${action('field-added')}>
      </frontmatter-editor-dialog>
    `;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    
    // Find the dialog element
    const dialog = canvasElement.querySelector('frontmatter-editor-dialog');
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveProperty('open', true);
    
    // Wait for the dialog to fully render
    await new Promise(resolve => setTimeout(resolve, 500));
    
    // Find inputs within the shadow DOM - this is more complex with web components
    // We'll verify the dialog is open and has the expected properties
    expect(dialog).toHaveProperty('loading', false);
    expect(dialog?.workingFrontmatter).toHaveProperty('title', 'Edit Me!');
    expect(dialog?.workingFrontmatter).toHaveProperty('published', false);
    
    // Note: Interacting with shadow DOM elements requires more complex selectors
    // This play function demonstrates property verification rather than DOM interaction
  },
  parameters: {
    docs: {
      description: {
        story: 'This story provides an interactive frontmatter editor for testing form interactions. The play function verifies the dialog state and properties. For manual testing, try editing field values, changing keys, adding new fields, and using the save/cancel buttons. Watch both the Interactions panel and browser console (F12) for all triggered events and their data.',
      },
    },
  },
};