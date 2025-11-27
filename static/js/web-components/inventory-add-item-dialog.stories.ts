/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './inventory-add-item-dialog.js';

const meta: Meta = {
  title: 'Components/Inventory/AddItemDialog',
  tags: ['autodocs'],
  component: 'inventory-add-item-dialog',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
Modal dialog for adding new inventory items to a container.

**Features:**
- Pre-filled readonly container field
- Item identifier input (required)
- Optional title field for human-readable name
- Form validation prevents empty submissions
- Loading state during submission
- Error display for failed operations

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('inventory-add-item-dialog');
dialog.openDialog('drawer_kitchen');
\`\`\`
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('inventory-add-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('drawer_kitchen');
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Add Item Dialog</h3>
        <p>Add a new item to a container.</p>
        <button @click=${openDialog}>Open Add Item Dialog</button>
        <inventory-add-item-dialog></inventory-add-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default add item dialog with container pre-filled. Enter an item identifier to enable the Add button.',
      },
    },
  },
};

export const WithPrefilledData: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('inventory-add-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('toolbox_garage');
        // Pre-fill some data for demonstration
        setTimeout(() => {
          dialog.itemIdentifier = 'screwdriver';
          dialog.itemTitle = 'Phillips Head Screwdriver';
        }, 50);
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Pre-filled Add Item Dialog</h3>
        <p>Dialog with sample data already filled in.</p>
        <button @click=${openDialog}>Open Pre-filled Dialog</button>
        <inventory-add-item-dialog></inventory-add-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with sample data pre-filled for demonstration purposes.',
      },
    },
  },
};

export const LoadingState: Story = {
  render: () => {
    const openDialogWithLoading = () => {
      const dialog = document.querySelector('inventory-add-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('box_storage');
        dialog.itemIdentifier = 'hammer';
        dialog.itemTitle = 'Claw Hammer';
        // Simulate loading state
        setTimeout(() => {
          dialog.loading = true;
        }, 100);
      }
    };

    setTimeout(openDialogWithLoading, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Loading State</h3>
        <p>Dialog during item creation with loading indicator.</p>
        <button @click=${openDialogWithLoading}>Open Loading Dialog</button>
        <inventory-add-item-dialog></inventory-add-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Buttons are disabled and show "Adding..." text during submission.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading state when an item is being created. The Add button shows "Adding..." and all inputs are disabled.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const openDialogWithError = () => {
      const dialog = document.querySelector('inventory-add-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('drawer_kitchen');
        dialog.itemIdentifier = 'screwdriver';
        // Simulate error state
        setTimeout(() => {
          dialog.error = 'Item "screwdriver" already exists in this container';
        }, 100);
      }
    };

    setTimeout(openDialogWithError, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Error State</h3>
        <p>Dialog showing error message from failed creation.</p>
        <button @click=${openDialogWithError}>Open Error Dialog</button>
        <inventory-add-item-dialog></inventory-add-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Error messages appear at the top of the form content area.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when item creation fails. The error message is displayed prominently above the form fields.',
      },
    },
  },
};

export const ValidationDisabled: Story = {
  render: () => {
    const openDialogEmpty = () => {
      const dialog = document.querySelector('inventory-add-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('shelf_basement');
        // Leave item identifier empty
        dialog.itemIdentifier = '';
      }
    };

    setTimeout(openDialogEmpty, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Validation - Button Disabled</h3>
        <p>Add button is disabled when item identifier is empty.</p>
        <button @click=${openDialogEmpty}>Open Empty Dialog</button>
        <inventory-add-item-dialog></inventory-add-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Type an item identifier to enable the Add button.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates form validation - the Add button remains disabled until a valid item identifier is entered.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const containers = ['drawer_kitchen', 'toolbox_garage', 'shelf_basement', 'box_storage'];

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 500px;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click any container button to open dialog with that container</li>
          <li>Test form validation by leaving item identifier empty</li>
          <li>Try pressing Escape to close dialog</li>
          <li>Click outside dialog backdrop to dismiss</li>
          <li>Test keyboard navigation with Tab key</li>
        </ul>

        <div style="display: flex; gap: 10px; margin: 15px 0; flex-wrap: wrap;">
          ${containers.map(container => html`
            <button @click=${() => {
              const dialog = document.querySelector('inventory-add-item-dialog') as any;
              if (dialog) {
                dialog.openDialog(container);
              }
            }}>
              Add to ${container}
            </button>
          `)}
        </div>

        <inventory-add-item-dialog></inventory-add-item-dialog>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Container field is readonly and pre-filled</li>
            <li>✅ Add button disabled when item identifier empty</li>
            <li>✅ Escape key closes dialog</li>
            <li>✅ Backdrop click closes dialog</li>
            <li>✅ Form inputs are focused for keyboard navigation</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Note:</strong> Actual item creation requires backend connection.
          This story demonstrates UI behavior only.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing of the add item dialog. Test various containers, validation states, and interaction patterns.',
      },
    },
  },
};
