/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import sinon from 'sinon';
import './insert-new-page-dialog.js';
import type { InsertNewPageDialog } from './insert-new-page-dialog.js';

const meta: Meta = {
  title: 'Components/InsertNewPageDialog',
  tags: ['autodocs'],
  component: 'insert-new-page-dialog',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A modal dialog for creating new wiki pages with template support.

**Key Features:**
- Title input with auto-generated identifier (automagic mode)
- Toggle between automagic and manual identifier entry
- Template dropdown populated from pages with \`template: true\` frontmatter
- Embedded frontmatter editor
- Dropdown locks when template selected or frontmatter edited
- Inline confirmation to clear frontmatter when changing template
- Dispatches \`page-created\` event with markdown link

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('insert-new-page-dialog');
await dialog.openDialog();

// Listen for page creation
dialog.addEventListener('page-created', (e) => {
  const { identifier, title, markdownLink } = e.detail;
  // Insert markdownLink at cursor position
  editor.insertText(markdownLink);
});
\`\`\`

**Event Detail:**
\`\`\`typescript
{
  identifier: 'my_new_article',
  title: 'My New Article',
  markdownLink: '[My New Article](/my_new_article)'
}
\`\`\`
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

function setupMocks(dialog: InsertNewPageDialog, options: {
  templates?: Array<{ identifier: string; title: string; description: string }>;
  generateIdentifierResponse?: { identifier: string; isUnique: boolean; existingPage?: any };
  createPageSuccess?: boolean;
} = {}) {
  const {
    templates = [
      { identifier: 'article_template', title: 'Article Template', description: 'Standard article format' },
      { identifier: 'project_template', title: 'Project Template', description: 'Project documentation' },
      { identifier: 'recipe_template', title: 'Recipe Template', description: 'Cooking recipe format' },
    ],
    generateIdentifierResponse = { identifier: '', isUnique: true },
    createPageSuccess = true,
  } = options;

  const pageCreator = (dialog as any).pageCreator;

  // Stub listTemplates
  sinon.stub(pageCreator, 'listTemplates').resolves({ templates });

  // Stub generateIdentifier
  const generateIdentifierStub = sinon.stub(pageCreator, 'generateIdentifier');
  generateIdentifierStub.callsFake(async (textArg: unknown) => {
    // Simulate identifier generation
    const text = textArg as string;
    const identifier = text.toLowerCase().replace(/\s+/g, '_').replace(/[^a-z0-9_]/g, '');
    return {
      identifier,
      isUnique: generateIdentifierResponse.isUnique,
      existingPage: generateIdentifierResponse.existingPage,
    };
  });

  // Stub createPage
  sinon.stub(pageCreator, 'createPage').resolves({
    success: createPageSuccess,
    error: createPageSuccess ? undefined : new Error('Failed to create page'),
  });

  // Stub showSuccess
  sinon.stub(pageCreator, 'showSuccess');
}

export const Default: Story = {
  render: () => {
    const openDialog = async () => {
      const dialog = document.querySelector('insert-new-page-dialog') as InsertNewPageDialog;
      if (dialog) {
        setupMocks(dialog);
        await dialog.openDialog();
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Insert New Page Dialog</h3>
        <p>Dialog with title-first workflow and template support.</p>
        <button @click=${openDialog}>Open Dialog</button>
        <insert-new-page-dialog
          @page-created=${action('page-created')}
        ></insert-new-page-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default dialog state with templates loaded. Enter a title and the identifier auto-generates.',
      },
    },
  },
};

export const NoTemplatesAvailable: Story = {
  render: () => {
    const openDialog = async () => {
      const dialog = document.querySelector('insert-new-page-dialog') as InsertNewPageDialog;
      if (dialog) {
        setupMocks(dialog, { templates: [] });
        await dialog.openDialog();
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>No Templates Available</h3>
        <p>When no pages have <code>template: true</code> in frontmatter.</p>
        <button @click=${openDialog}>Open Dialog</button>
        <insert-new-page-dialog
          @page-created=${action('page-created')}
        ></insert-new-page-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Notice the template dropdown is disabled with a note.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the disabled template dropdown when no templates are defined.',
      },
    },
  },
};

export const IdentifierConflict: Story = {
  render: () => {
    const openDialog = async () => {
      const dialog = document.querySelector('insert-new-page-dialog') as InsertNewPageDialog;
      if (dialog) {
        setupMocks(dialog, {
          generateIdentifierResponse: {
            identifier: 'existing_page',
            isUnique: false,
            existingPage: {
              identifier: 'existing_page',
              title: 'Existing Page Title',
            },
          },
        });
        await dialog.openDialog();

        // Pre-fill with conflicting identifier
        setTimeout(() => {
          dialog.pageTitle = 'Existing Page';
          dialog.pageIdentifier = 'existing_page';
          dialog.isUnique = false;
        }, 200);
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Identifier Conflict</h3>
        <p>When the generated identifier matches an existing page.</p>
        <button @click=${openDialog}>Open Dialog</button>
        <insert-new-page-dialog
          @page-created=${action('page-created')}
        ></insert-new-page-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The Create Page button is disabled when an identifier conflict exists.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the conflict warning when an identifier already exists.',
      },
    },
  },
};

export const TemplateSelected: Story = {
  render: () => {
    const openDialog = async () => {
      const dialog = document.querySelector('insert-new-page-dialog') as InsertNewPageDialog;
      if (dialog) {
        setupMocks(dialog);
        await dialog.openDialog();

        // Simulate template selection
        setTimeout(() => {
          dialog.selectedTemplate = 'article_template';
          dialog.templateLocked = true;
        }, 200);
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Template Selected</h3>
        <p>After selecting a template, the dropdown is locked.</p>
        <button @click=${openDialog}>Open Dialog</button>
        <insert-new-page-dialog
          @page-created=${action('page-created')}
        ></insert-new-page-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The "Change" button appears. Click it to see the inline confirmation.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the locked dropdown state after selecting a template.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const openDialog = async () => {
      const dialog = document.querySelector('insert-new-page-dialog') as InsertNewPageDialog;
      if (dialog) {
        setupMocks(dialog);
        await dialog.openDialog();
      }
    };

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Interactive Testing</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Enter a title and watch the identifier auto-generate</li>
          <li>Click the sparkle/pen button to toggle manual identifier mode</li>
          <li>Select a template and observe the dropdown lock</li>
          <li>Click "Change" and confirm to unlock the dropdown</li>
          <li>Add frontmatter fields and verify the dropdown locks</li>
          <li>Click Create Page to see the page-created event</li>
        </ul>

        <button @click=${openDialog} style="margin: 15px 0; padding: 10px 20px;">
          Open Dialog
        </button>

        <insert-new-page-dialog
          @page-created=${action('page-created')}
        ></insert-new-page-dialog>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>Title input auto-generates identifier (debounced)</li>
            <li>Template dropdown shows available templates</li>
            <li>Dropdown locks when template selected OR frontmatter edited</li>
            <li>"Change" button clears frontmatter with confirmation</li>
            <li>Create button enabled only when identifier is unique</li>
            <li>page-created event includes markdown link</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing for all dialog features. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};
