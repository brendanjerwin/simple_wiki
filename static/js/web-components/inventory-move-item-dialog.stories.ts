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
Modal dialog for moving inventory items between containers using search-based destination selection.

**Features:**
- Readonly item identifier and current container fields
- Search field for finding destination containers
- Search results appear as "Move To" buttons
- Clicking a result immediately moves the item
- Loading state on individual buttons during move
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
        <p>Search for a destination container and click "Move To" to move the item.</p>
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
        story: 'Default move item dialog. Type in the search field to find destination containers.',
      },
    },
  },
};

export const WithSearchResults: Story = {
  render: () => {
    const openDialogWithResults = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('hammer', 'toolbox_garage');
        setTimeout(() => {
          dialog.searchQuery = 'shelf';
          dialog.searchResults = [
            {
              identifier: 'shelf_basement',
              title: 'Basement Shelf',
              fragment: '',
              highlights: [],
              frontmatter: { 'inventory.container': 'basement' },
            },
            {
              identifier: 'shelf_garage',
              title: 'Garage Shelf',
              fragment: '',
              highlights: [],
              frontmatter: {},
            },
            {
              identifier: 'shelf_kitchen',
              title: 'Kitchen Pantry Shelf',
              fragment: '',
              highlights: [],
              frontmatter: { 'inventory.container': 'kitchen' },
            },
          ];
        }, 50);
      }
    };

    setTimeout(openDialogWithResults, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Move Dialog with Search Results</h3>
        <p>Dialog showing search results for "shelf".</p>
        <button @click=${openDialogWithResults}>Open Dialog with Results</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Each result shows a "Move To" button. Click to move the item immediately.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with search results displayed. Each result includes a "Move To" button.',
      },
    },
  },
};

export const SearchLoading: Story = {
  render: () => {
    const openDialogSearchLoading = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('wrench', 'drawer_kitchen');
        setTimeout(() => {
          dialog.searchQuery = 'toolbox';
          dialog.searchLoading = true;
        }, 50);
      }
    };

    setTimeout(openDialogSearchLoading, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Search Loading State</h3>
        <p>Dialog during search operation.</p>
        <button @click=${openDialogSearchLoading}>Open Search Loading Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Shows "Searching for containers..." while the search is in progress.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the search loading state when searching for destination containers.',
      },
    },
  },
};

export const MovingInProgress: Story = {
  render: () => {
    const openDialogMoving = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('pliers', 'toolbox_garage');
        setTimeout(() => {
          dialog.searchQuery = 'drawer';
          dialog.searchResults = [
            {
              identifier: 'drawer_kitchen',
              title: 'Kitchen Drawer',
              fragment: '',
              highlights: [],
              frontmatter: { 'inventory.container': 'kitchen' },
            },
            {
              identifier: 'drawer_bedroom',
              title: 'Bedroom Drawer',
              fragment: '',
              highlights: [],
              frontmatter: {},
            },
          ];
          dialog.movingTo = 'drawer_kitchen';
        }, 100);
      }
    };

    setTimeout(openDialogMoving, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Moving In Progress</h3>
        <p>Dialog during move operation.</p>
        <button @click=${openDialogMoving}>Open Moving Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The clicked button shows "Moving..." and all inputs are disabled.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the moving state when an item is being moved. The active button shows "Moving..." and all other controls are disabled.',
      },
    },
  },
};

export const NoResults: Story = {
  render: () => {
    const openDialogNoResults = () => {
      const dialog = document.querySelector('inventory-move-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('screwdriver', 'drawer_kitchen');
        setTimeout(() => {
          dialog.searchQuery = 'nonexistent';
          dialog.searchResults = [];
          dialog.searchLoading = false;
        }, 50);
      }
    };

    setTimeout(openDialogNoResults, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>No Results State</h3>
        <p>Dialog when search returns no matching containers.</p>
        <button @click=${openDialogNoResults}>Open No Results Dialog</button>
        <inventory-move-item-dialog></inventory-move-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Shows "No containers found" message when the search doesn't match any containers.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows what happens when a search returns no matching containers.',
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
        setTimeout(() => {
          dialog.searchQuery = 'toolbox';
          dialog.searchResults = [
            {
              identifier: 'toolbox_garage',
              title: 'Garage Toolbox',
              fragment: '',
              highlights: [],
              frontmatter: {},
            },
          ];
          dialog.error = 'Failed to move item: Permission denied';
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
          <li>Type in the search field to search for containers</li>
          <li>Click "Move To" buttons to trigger a move (requires backend)</li>
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
            <li>Item and current container fields are readonly</li>
            <li>Search field accepts input and triggers debounced search</li>
            <li>Search results appear with "Move To" buttons</li>
            <li>Current container is filtered from search results</li>
            <li>Escape key closes dialog</li>
            <li>Backdrop click closes dialog</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Note:</strong> Search and move operations require backend connection.
          This story demonstrates UI behavior only.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing of the move item dialog. Test various items, search functionality, and interaction patterns.',
      },
    },
  },
};
