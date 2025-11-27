/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './inventory-find-item-dialog.js';

const meta: Meta = {
  title: 'Components/Inventory/FindItemDialog',
  tags: ['autodocs'],
  component: 'inventory-find-item-dialog',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
Modal dialog for searching inventory item locations.

**Features:**
- Search input with Enter key support
- Displays container path with clickable links
- Shows anomaly warnings for items in multiple containers
- Not found message for items without container assignment
- Loading state during search
- Error display for failed operations

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('inventory-find-item-dialog');
dialog.openDialog();
// Or with pre-filled query:
dialog.openDialog('screwdriver');
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
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog();
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Find Item Dialog</h3>
        <p>Search for an item's location in the inventory.</p>
        <button @click=${openDialog}>Open Find Item Dialog</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Default find item dialog. Enter an item identifier and press Search or Enter.',
      },
    },
  },
};

export const WithPrefilledQuery: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('screwdriver');
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Pre-filled Search Query</h3>
        <p>Dialog opened with a search query already entered.</p>
        <button @click=${openDialog}>Open Pre-filled Dialog</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with a search query pre-filled, ready to search.',
      },
    },
  },
};

export const FoundInSingleLocation: Story = {
  render: () => {
    const openDialogWithResult = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('hammer');
        setTimeout(() => {
          dialog.results = {
            found: true,
            locations: [
              { container: 'toolbox_garage', path: ['house', 'garage', 'toolbox_garage'] },
            ],
            summary: 'Found hammer in toolbox_garage',
          };
        }, 50);
      }
    };

    setTimeout(openDialogWithResult, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Found in Single Location</h3>
        <p>Item found in one container - click the link to navigate.</p>
        <button @click=${openDialogWithResult}>Show Single Result</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the results when an item is found in a single container location.',
      },
    },
  },
};

export const FoundInMultipleLocations: Story = {
  render: () => {
    const openDialogWithResults = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('screwdriver');
        setTimeout(() => {
          dialog.results = {
            found: true,
            locations: [
              { container: 'drawer_kitchen', path: ['house', 'kitchen', 'drawer_kitchen'] },
              { container: 'toolbox_garage', path: ['house', 'garage', 'toolbox_garage'] },
            ],
            summary: 'Found screwdriver in 2 locations (anomaly)',
          };
        }, 50);
      }
    };

    setTimeout(openDialogWithResults, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 500px;">
        <h3>Found in Multiple Locations (Anomaly)</h3>
        <p>Item found in multiple containers - shows warning.</p>
        <button @click=${openDialogWithResults}>Show Multiple Results</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Items should normally exist in only one container. Multiple locations indicate an anomaly.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the anomaly warning when an item is found in multiple containers. This indicates data inconsistency.',
      },
    },
  },
};

export const ItemNotFound: Story = {
  render: () => {
    const openDialogNotFound = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('nonexistent_item');
        setTimeout(() => {
          dialog.results = {
            found: false,
            locations: [],
            summary: 'Item not found',
          };
        }, 50);
      }
    };

    setTimeout(openDialogNotFound, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Item Not Found</h3>
        <p>Item has no container assignment.</p>
        <button @click=${openDialogNotFound}>Show Not Found</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the "not found" message when an item has no container assignment.',
      },
    },
  },
};

export const LoadingState: Story = {
  render: () => {
    const openDialogLoading = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('wrench');
        dialog.searchQuery = 'wrench';
        setTimeout(() => {
          dialog.loading = true;
        }, 100);
      }
    };

    setTimeout(openDialogLoading, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Loading State</h3>
        <p>Dialog during search operation.</p>
        <button @click=${openDialogLoading}>Open Loading Dialog</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Search button shows "Searching..." and input is disabled.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading state while searching for an item.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const openDialogWithError = () => {
      const dialog = document.querySelector('inventory-find-item-dialog') as any;
      if (dialog) {
        dialog.openDialog('hammer');
        setTimeout(() => {
          dialog.error = 'Failed to search: Server unavailable';
        }, 100);
      }
    };

    setTimeout(openDialogWithError, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Error State</h3>
        <p>Dialog showing search error.</p>
        <button @click=${openDialogWithError}>Open Error Dialog</button>
        <inventory-find-item-dialog></inventory-find-item-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error state when a search operation fails.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const scenarios = [
      { name: 'Empty Search', query: '' },
      { name: 'Single Result', query: 'hammer' },
      { name: 'Multiple Results', query: 'screwdriver' },
      { name: 'Not Found', query: 'unicorn' },
    ];

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 500px;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click buttons to open dialog with different pre-filled queries</li>
          <li>Try pressing Enter in the search field to search</li>
          <li>Test Escape key to close dialog</li>
          <li>Click outside dialog backdrop to dismiss</li>
          <li>Click location links to test navigation</li>
        </ul>

        <div style="display: flex; gap: 10px; margin: 15px 0; flex-wrap: wrap;">
          ${scenarios.map(({ name, query }) => html`
            <button @click=${() => {
              const dialog = document.querySelector('inventory-find-item-dialog') as any;
              if (dialog) {
                dialog.openDialog(query);
              }
            }}>
              ${name}
            </button>
          `)}
        </div>

        <inventory-find-item-dialog></inventory-find-item-dialog>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Search button disabled when query is empty</li>
            <li>✅ Enter key triggers search</li>
            <li>✅ Escape key closes dialog</li>
            <li>✅ Backdrop click closes dialog</li>
            <li>✅ Location links are clickable</li>
            <li>✅ Anomaly warning for multiple locations</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Note:</strong> Actual search requires backend connection.
          This story demonstrates UI behavior only.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing of the find item dialog. Test various scenarios and interaction patterns.',
      },
    },
  },
};
