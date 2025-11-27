/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './inventory-move-item-dialog.js';

const meta: Meta = {
  title: 'Components/Inventory/MoveItemDialog',
  tags: ['autodocs'],
  component: 'inventory-move-item-dialog',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
Modal dialog for moving inventory items between containers.

**Features:**
- Readonly item identifier and current container fields
- New container input (required)
- Validation prevents moving to same container
- Loading state during submission
- Error display for failed operations

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('inventory-move-item-dialog');
dialog.openDialog('screwdriver', 'drawer_kitchen');
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
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('screwdriver', 'drawer_kitchen');
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Move Item Dialog</h3>
        <p>Move an item from one container to another.</p>
        <button @click=${openDialog}>Open Move Item Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default move item dialog with item and current container pre-filled. Enter a new container to enable the Move button.',
      },
    },
  },
};

export const WithDestination: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('hammer', 'toolbox_garage');
        setTimeout(() => {
          dialog.newContainer = 'shelf_basement';
        }, 50);
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Move Dialog with Destination</h3>
        <p>Dialog with destination container already specified.</p>
        <button @click=${openDialog}>Open Pre-filled Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with a destination container already filled in, ready to submit.',
      },
    },
  },
};

export const LoadingState: Story = {
  render: () => {
    const openDialogWithLoading = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('wrench', 'drawer_kitchen');
        dialog.newContainer = 'toolbox_garage';
        setTimeout(() => {
          dialog.loading = true;
        }, 100);
      }
    };

    setTimeout(openDialogWithLoading, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Loading State</h3>
        <p>Dialog during move operation with loading indicator.</p>
        <button @click=${openDialogWithLoading}>Open Loading Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Button shows "Moving..." and inputs are disabled during submission.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading state when an item is being moved. The Move button shows "Moving..." and all inputs are disabled.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const openDialogWithError = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('screwdriver', 'drawer_kitchen');
        dialog.newContainer = 'nonexistent_container';
        setTimeout(() => {
          dialog.error = 'Container "nonexistent_container" not found';
        }, 100);
      }
    };

    setTimeout(openDialogWithError, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Error State</h3>
        <p>Dialog showing error message from failed move.</p>
        <button @click=${openDialogWithError}>Open Error Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Error messages appear at the top of the form content area.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when a move operation fails. The error message is displayed prominently above the form fields.',
      },
    },
  },
};

export const SameContainerValidation: Story = {
  render: () => {
    const openDialogSameContainer = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('pliers', 'toolbox_garage');
        setTimeout(() => {
          dialog.newContainer = 'toolbox_garage';
        }, 50);
      }
    };

    setTimeout(openDialogSameContainer, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Same Container Validation</h3>
        <p>Move button is disabled when destination is same as current.</p>
        <button @click=${openDialogSameContainer}>Open Same Container Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The Move button remains disabled because the destination equals the current container.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates validation that prevents moving an item to the same container it is already in.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const items = [
      { item: 'screwdriver', container: 'drawer_kitchen' },
      { item: 'hammer', container: 'toolbox_garage' },
      { item: 'wrench', container: 'shelf_basement' },
      { item: 'pliers', container: 'box_storage' },
    ];

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 500px;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click any item button to open the move dialog</li>
          <li>Test validation by entering same container as current</li>
          <li>Try pressing Escape to close dialog</li>
          <li>Click outside dialog backdrop to dismiss</li>
          <li>Test keyboard navigation with Tab key</li>
        </ul>

        <div style="display: flex; gap: 10px; margin: 15px 0; flex-wrap: wrap;">
          ${items.map(({ item, container }) => html`
            <button @click=${() => {
              const dialog = document.querySelector('inventory-move-item-dialog') as any;
              if (dialog) {
                dialog.openDialog(item, container);
              }
            }}>
              Move ${item}
            </button>
          `)}
        </div>

        <inventory-move-item-dialog></inventory-move-item-dialog>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Item and current container fields are readonly</li>
            <li>✅ Move button disabled when destination is empty</li>
            <li>✅ Move button disabled when destination equals current</li>
            <li>✅ Escape key closes dialog</li>
            <li>✅ Backdrop click closes dialog</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Note:</strong> Actual move operation requires backend connection.
          This story demonstrates UI behavior only.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing of the move item dialog. Test various items, validation states, and interaction patterns.',
      },
    },
  },
};
