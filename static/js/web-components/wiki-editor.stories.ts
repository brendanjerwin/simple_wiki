import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './wiki-editor.js';
import { AugmentErrorService } from './augment-error-service.js';
import type { WikiEditor } from './wiki-editor.js';

const meta: Meta = {
  title: 'Components/WikiEditor',
  tags: ['autodocs'],
  component: 'wiki-editor',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A full-page markdown editor backed by the gRPC PageManagementService API.

**Features:**
- Loads content via ReadPage gRPC call
- Auto-saves with debounced UpdateWholePage calls
- Concurrent-save prevention (queues saves while one is in flight)
- Status bar showing save state (Editing / Saving / Saved / Error)
- Tab key inserts tab characters
- File upload via drag-and-drop (file-drop-zone integration)
- Context menu and toolbar integration for formatting

**Usage:**
\`\`\`html
<wiki-editor page="my-page" allow-uploads max-upload-mb="10" debounce-ms="750"></wiki-editor>
\`\`\`

**Storybook note:** In Storybook, the component has no backend, so stories
bypass the API by setting internal properties directly after creation.
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

function setEditorState(
  el: WikiEditor,
  overrides: {
    loading?: boolean;
    content?: string;
    saveStatus?: string;
    error?: unknown;
  }
): void {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  const internal = el as unknown as Record<string, unknown>;
  if (overrides.loading !== undefined) internal['loading'] = overrides.loading;
  if (overrides.content !== undefined) internal['content'] = overrides.content;
  if (overrides.saveStatus !== undefined) internal['saveStatus'] = overrides.saveStatus;
  if (overrides.error !== undefined) internal['error'] = overrides.error;
}

const sampleContent = `+++
title = "Sample Page"
tags = ["wiki", "example"]
+++
# Sample Page

This is a sample wiki page with some **bold** and *italic* text.

## Lists

- Item one
- Item two
- Item three

## Code

\`\`\`javascript
function hello() {
  console.log("Hello, world!");
}
\`\`\`
`;

// ---------------------------------------------------------------------------
// Stories
// ---------------------------------------------------------------------------

export const Loading: Story = {
  render: () => {
    return html`
      <div style="height: 400px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor in its initial loading state while fetching content from the API.',
      },
    },
  },
};

export const WithContent: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: sampleContent,
          saveStatus: 'idle',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor with content loaded and ready for editing.',
      },
    },
  },
};

export const Editing: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: sampleContent,
          saveStatus: 'editing',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor while the user is actively typing (before debounce fires).',
      },
    },
  },
};

export const Saving: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: sampleContent,
          saveStatus: 'saving',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor while an auto-save request is in flight.',
      },
    },
  },
};

export const Saved: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: sampleContent,
          saveStatus: 'saved',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor after a successful save, showing the "Saved" indicator.',
      },
    },
  },
};

export const SaveError: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: sampleContent,
          saveStatus: 'error',
          error: AugmentErrorService.augmentError(
            new Error('Failed to save: conflict detected'),
            'save page'
          ),
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor showing a save error in the status bar. The textarea remains accessible so the user can retry.',
      },
    },
  },
};

export const LoadError: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          saveStatus: 'idle',
          error: AugmentErrorService.augmentError(
            new Error('Network error: connection refused'),
            'load page content'
          ),
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor when the initial content load fails. Shows error-display component instead of the textarea.',
      },
    },
  },
};

export const WithUploads: Story = {
  render: () => {
    const template = html`
      <div style="height: 500px;">
        <wiki-editor page="test-page" allow-uploads max-upload-mb="10"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: '# Page with uploads\n\nDrag and drop files onto the editor.',
          saveStatus: 'idle',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: 'The editor with file uploads enabled. Drop files onto the textarea to upload and insert markdown links.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const template = html`
      <div style="height: 600px;">
        <wiki-editor page="test-page" allow-uploads debounce-ms="1000"></wiki-editor>
      </div>
    `;
    setTimeout(() => {
      const el = document.querySelector('wiki-editor') as WikiEditor;
      if (el) {
        setEditorState(el, {
          loading: false,
          content: '# Interactive Test\n\nTry typing here.\n\nPress Tab to insert a tab character.\n\nWatch the status bar as you type.',
          saveStatus: 'idle',
        });
      }
    }, 0);
    return template;
  },
  parameters: {
    docs: {
      description: {
        story: `Interactive testing story for manual verification.

**Test steps:**
1. Type in the textarea — status bar should show "Editing"
2. Press Tab — a tab character should be inserted
3. Open the browser developer tools console to see action logs
4. Note: Without a backend, saves will fail — this is expected in Storybook`,
      },
    },
  },
};
