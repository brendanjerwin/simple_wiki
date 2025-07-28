import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import './confirmation-dialog.js';
import { AugmentErrorService } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/ConfirmationDialog',
  tags: ['autodocs'],
  component: 'confirmation-dialog',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A generic confirmation dialog component that can be reused for various confirmation scenarios.

**Features:**
- Configurable message, buttons, and styling
- Loading states during async operations  
- Error display integration
- Keyboard shortcuts (Enter to confirm, Escape to cancel)
- Click-outside-to-close functionality
- Multiple button variants (primary, danger, warning)

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('confirmation-dialog');
dialog.openDialog({
  message: 'Are you sure you want to delete this item?',
  description: 'This action cannot be undone.',
  confirmText: 'Delete',
  confirmVariant: 'danger'
});
\`\`\`
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const Closed: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <p>The dialog is closed. Use other stories to see it in different states.</p>
      <confirmation-dialog></confirmation-dialog>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'The default state when the dialog is closed and hidden.',
      },
    },
  },
};

export const DangerConfirmation: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Are you sure you want to delete this page?',
          description: 'Page: home',
          confirmText: 'Delete Page',
          cancelText: 'Cancel',
          confirmVariant: 'danger',
          icon: 'warning',
          irreversible: true
        });
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Danger Confirmation Example</h3>
        <p>This story shows a typical destructive action confirmation.</p>
        <button @click=${openDialog}>Open Delete Confirmation</button>
        <confirmation-dialog
          @confirm=${action('confirm-delete')}
          @cancel=${action('cancel-delete')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Open the browser developer tools console to see the action logs.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A typical destructive action confirmation with danger styling. Shows warning icon, irreversible action messaging, and red delete button. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const PrimaryConfirmation: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Do you want to save your changes?',
          description: 'Your changes will be permanently saved.',
          confirmText: 'Save Changes',
          cancelText: 'Discard',
          confirmVariant: 'primary',
          icon: '💾',
          irreversible: false
        });
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Primary Action Confirmation</h3>
        <p>This story shows a positive action confirmation.</p>
        <button @click=${openDialog}>Open Save Confirmation</button>
        <confirmation-dialog
          @confirm=${action('confirm-save')}
          @cancel=${action('cancel-save')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Open the browser developer tools console to see the action logs.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A positive action confirmation with primary styling. Shows save icon and blue confirm button. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const WarningConfirmation: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'This action may have side effects',
          description: 'Proceeding will regenerate all cache files and may take several minutes.',
          confirmText: 'Proceed Anyway',
          cancelText: 'Cancel',
          confirmVariant: 'warning',
          icon: 'warning',
          irreversible: false
        });
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Warning Confirmation Example</h3>
        <p>This story shows a cautionary action confirmation.</p>
        <button @click=${openDialog}>Open Warning Confirmation</button>
        <confirmation-dialog
          @confirm=${action('confirm-proceed')}
          @cancel=${action('cancel-proceed')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Open the browser developer tools console to see the action logs.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A cautionary action confirmation with warning styling. Shows orange/yellow confirm button for actions that need attention but are not destructive. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const LoadingState: Story = {
  render: () => {
    const openDialogWithLoading = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Are you sure you want to delete this item?',
          confirmText: 'Delete',
          confirmVariant: 'danger',
          irreversible: true
        });
        
        // Simulate loading state
        setTimeout(() => {
          dialog.setLoading(true);
        }, 500);
      }
    };

    setTimeout(openDialogWithLoading, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Loading State Example</h3>
        <p>This story shows the dialog in a loading state with disabled buttons.</p>
        <button @click=${openDialogWithLoading}>Open Dialog with Loading</button>
        <confirmation-dialog
          @confirm=${action('confirm-while-loading')}
          @cancel=${action('cancel-while-loading')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The dialog will enter loading state after opening. Buttons become disabled and show loading text.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog in loading state with disabled buttons and loading text. This is useful during async operations to prevent multiple submissions.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const openDialogWithError = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Are you sure you want to delete this item?',
          confirmText: 'Delete',
          confirmVariant: 'danger',
          irreversible: true
        });
        
        // Simulate error state
        setTimeout(() => {
          const mockError = new Error('Network connection failed');
          const augmentedError = AugmentErrorService.augmentError(mockError, 'delete item');
          dialog.showError(augmentedError);
        }, 500);
      }
    };

    setTimeout(openDialogWithError, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Error State Example</h3>
        <p>This story shows the dialog displaying an error message.</p>
        <button @click=${openDialogWithError}>Open Dialog with Error</button>
        <confirmation-dialog
          @confirm=${action('confirm-with-error')}
          @cancel=${action('cancel-with-error')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          The dialog will show an error after opening. This allows users to see what went wrong and potentially retry.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with an error message displayed. The dialog remains open so users can see the error and potentially retry the action.',
      },
    },
  },
};

export const MinimalConfiguration: Story = {
  render: () => {
    const openMinimalDialog = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Are you sure?'
        });
      }
    };

    setTimeout(openMinimalDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Minimal Configuration Example</h3>
        <p>This story shows the dialog with minimal configuration - just a message.</p>
        <button @click=${openMinimalDialog}>Open Minimal Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-minimal')}
          @cancel=${action('cancel-minimal')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Uses default settings: warning icon, "Confirm"/"Cancel" buttons, danger variant.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog with only a message configured. All other options use sensible defaults.',
      },
    },
  },
};

export const InteractiveKeyboardTesting: Story = {
  render: () => {
    const openInteractiveDialog = () => {
      const dialog = document.querySelector('confirmation-dialog');
      if (dialog) {
        dialog.openDialog({
          message: 'Test keyboard shortcuts',
          description: 'Try pressing Escape to cancel or Ctrl+Enter to confirm',
          confirmText: 'Confirm (Ctrl+Enter)',
          cancelText: 'Cancel (Esc)',
          confirmVariant: 'primary'
        });
      }
    };

    setTimeout(openInteractiveDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Interactive Keyboard Testing</h3>
        <p>This story demonstrates keyboard shortcuts and interaction patterns.</p>
        <button @click=${openInteractiveDialog}>Open Interactive Dialog</button>
        <confirmation-dialog
          @confirm=${action('keyboard-confirm')}
          @cancel=${action('keyboard-cancel')}>
        </confirmation-dialog>
        <div style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <p><strong>Test these interactions:</strong></p>
          <ul>
            <li>Press <code>Escape</code> to cancel</li>
            <li>Press <code>Ctrl+Enter</code> (or <code>Cmd+Enter</code> on Mac) to confirm</li>
            <li>Click outside the dialog to cancel</li>
            <li>Click the buttons normally</li>
          </ul>
          <p>Open the browser developer tools console to see the action logs.</p>
        </div>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive story for testing keyboard shortcuts and user interaction patterns. Test Escape to cancel, Ctrl+Enter to confirm, and click-outside-to-close. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};