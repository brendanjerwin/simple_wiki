/* eslint-disable @typescript-eslint/no-explicit-any */
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
A compact confirmation dialog styled to match the system-info design language.

**Key Visual Features:**
- Compact typography with monospace fonts for technical content
- System-info color palette and spacing patterns
- Smooth micro-animations and hover effects
- Backdrop blur for modern glass effect
- Mobile-responsive design with scaled typography

**Technical Features:**
- Configurable message, buttons, and styling variants
- Loading states during async operations  
- Error display integration with AugmentedError
- Click-outside-to-close functionality
- Multiple button variants (primary, danger, warning)

**Usage:**
\`\`\`typescript
const dialog = document.querySelector('confirmation-dialog');
dialog.openDialog({
  message: 'Delete index: frontmatter?',
  description: 'Pages: 1,247 items',
  confirmText: 'Delete',
  confirmVariant: 'danger',
  irreversible: true
});
\`\`\`
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const CompactSystemInfoStyle: Story = {
  render: () => {
    const openCompactDialog = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Rebuild search index?',
          description: 'Current: 1,247 pages indexed',
          confirmText: 'Rebuild',
          cancelText: 'Cancel',
          confirmVariant: 'danger',
          icon: 'warning',
          irreversible: false
        });
      }
    };

    setTimeout(openCompactDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Compact System-Info Styling</h3>
        <p>Dialog with technical monospace typography and tight spacing</p>
        <button @click=${openCompactDialog}>Open System Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-rebuild')}
          @cancel=${action('cancel-rebuild')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Notice the compact design, monospace fonts, and system-info color palette.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the new compact system-info styling with technical typography, tight spacing, and system colors.',
      },
    },
  },
};

export const CompactDangerConfirmation: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Delete page: home.md?',
          description: 'Size: 12.4KB, Modified: 2min ago',
          confirmText: 'Delete',
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
        <h3>Compact Danger Confirmation</h3>
        <p>Destructive action with technical file information</p>
        <button @click=${openDialog}>Delete File Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-delete-file')}
          @cancel=${action('cancel-delete-file')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Destructive action dialog with system-info styling. Features technical file information with monospace typography and compact layout. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const CompactPrimaryAction: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Deploy build: v1.4.2?',
          description: 'Target: production, Size: 2.1MB',
          confirmText: 'Deploy',
          cancelText: 'Cancel',
          confirmVariant: 'primary',
          icon: 'system',
          irreversible: false
        });
      }
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Compact Primary Action</h3>
        <p>System deployment confirmation with technical details</p>
        <button @click=${openDialog}>Deploy Build Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-deploy')}
          @cancel=${action('cancel-deploy')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Primary actions use blue buttons with monospace technical information.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Primary action dialog featuring deployment confirmation. Shows technical details with monospace typography and compact system-info styling.',
      },
    },
  },
};

export const CompactWarningAction: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Clear cache: all indexes?',
          description: 'Duration: ~3min, Impact: Search unavailable',
          confirmText: 'Clear Cache',
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
        <h3>Compact Warning Action</h3>
        <p>Cache clearing with impact assessment</p>
        <button @click=${openDialog}>Clear Cache Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-clear-cache')}
          @cancel=${action('cancel-clear-cache')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Warning actions use yellow buttons (#ffc107) for potentially disruptive operations.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Warning action dialog for system maintenance operations. Features impact assessment with monospace typography and system-info color coding.',
      },
    },
  },
};

export const CompactLoadingState: Story = {
  render: () => {
    const openDialogWithLoading = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Reindex all pages?',
          description: 'Est. time: 2-3 minutes',
          confirmText: 'Start',
          confirmVariant: 'primary',
          irreversible: false
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
        <h3>Compact Loading State</h3>
        <p>Dialog in processing state with disabled compact buttons</p>
        <button @click=${openDialogWithLoading}>Start Reindex Process</button>
        <confirmation-dialog
          @confirm=${action('confirm-reindex')}
          @cancel=${action('cancel-reindex')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Loading state shows "Processing..." in compact monospace buttons.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows loading state with system-info compact styling. Buttons are disabled and show processing text in monospace font.',
      },
    },
  },
};

export const SystemErrorWithDetails: Story = {
  render: () => {
    const openDialogWithError = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Rebuild index: frontmatter?',
          description: 'Previous rebuild failed',
          confirmText: 'Retry',
          confirmVariant: 'warning',
          irreversible: false
        });
        
        // Simulate error state with system-specific error
        setTimeout(() => {
          const mockError = new Error('Insufficient disk space: 2.1GB required, 0.8GB available');
          const augmentedError = AugmentErrorService.augmentError(mockError, 'rebuild search index');
          dialog.showError(augmentedError);
        }, 500);
      }
    };

    setTimeout(openDialogWithError, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>System Error with Details</h3>
        <p>Dialog showing technical error with AugmentedError integration</p>
        <button @click=${openDialogWithError}>Trigger System Error</button>
        <confirmation-dialog
          @confirm=${action('confirm-retry')}
          @cancel=${action('cancel-retry')}>
        </confirmation-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Error display integrates with compact dialog styling. Click error to expand stack trace.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows error display integration with system-info styling. AugmentedError provides technical details while maintaining compact layout.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const scenarios = [
      {
        name: 'Quick Action',
        config: {
          message: 'Restart service?',
          description: 'Duration: <5s',
          confirmText: 'Restart',
          confirmVariant: 'primary' as const,
          icon: 'system' as const
        }
      },
      {
        name: 'Destructive Action',
        config: {
          message: 'Purge cache: all data?',
          description: 'WARNING: Cannot be undone',
          confirmText: 'Purge',
          confirmVariant: 'danger' as const,
          icon: 'warning' as const,
          irreversible: true
        }
      },
      {
        name: 'Maintenance',
        config: {
          message: 'Update dependencies?',
          description: 'Service downtime: ~2min',
          confirmText: 'Update',
          confirmVariant: 'warning' as const,
          icon: 'warning' as const
        }
      }
    ];

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Test different dialog variants and button styles</li>
          <li>Try clicking outside dialogs to dismiss</li>
          <li>Test keyboard navigation with Tab and Enter/Escape</li>
          <li>Notice hover animations on compact buttons</li>
          <li>Observe monospace typography for technical details</li>
        </ul>
        
        <div style="display: flex; gap: 10px; margin: 15px 0;">
          ${scenarios.map(({ name, config }) => html`
            <button @click=${() => {
              const dialog = document.querySelector('confirmation-dialog') as any;
              if (dialog) {
                dialog.openDialog(config);
              }
            }}>
              ${name}
            </button>
          `)}
        </div>
        
        <confirmation-dialog
          @confirm=${action('confirm-interactive')}
          @cancel=${action('cancel-interactive')}>
        </confirmation-dialog>
        
        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Compact system-info styling with monospace fonts</li>
            <li>✅ Smooth hover animations on buttons</li>
            <li>✅ Backdrop blur effect</li>
            <li>✅ Appropriate color coding by action type</li>
            <li>✅ Click outside to dismiss functionality</li>
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
        story: 'Comprehensive interactive testing of different dialog configurations. Test various scenarios, interaction patterns, and visual states. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const ResponsiveDesignDemo: Story = {
  render: () => {
    const openResponsiveDialog = () => {
      const dialog = document.querySelector('confirmation-dialog') as any;
      if (dialog) {
        dialog.openDialog({
          message: 'Mobile responsive test',
          description: 'Resize browser to see mobile adaptation: Smaller padding, reduced font sizes (12px→11px→10px), compact buttons (6px→4px padding)',
          confirmText: 'Test',
          confirmVariant: 'primary',
          irreversible: false
        });
      }
    };

    setTimeout(openResponsiveDialog, 100);

    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Responsive Design Test</h3>
        <p>Dialog adaptation for mobile devices:</p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li><strong>Desktop:</strong> 16px padding, 12px message font</li>
          <li><strong>Mobile (≤768px):</strong> 12px padding, 11px message font</li>
          <li><strong>Buttons:</strong> 6px→4px padding, 11px→10px font</li>
          <li><strong>Icons:</strong> 24px→20px size reduction</li>
        </ul>
        
        <button @click=${openResponsiveDialog}>Open Responsive Dialog</button>
        <confirmation-dialog
          @confirm=${action('confirm-responsive')}
          @cancel=${action('cancel-responsive')}>
        </confirmation-dialog>
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Resize browser window to test responsive breakpoints. System-info styling scales appropriately.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates responsive behavior with system-info styling. All elements scale appropriately for mobile while maintaining compact design principles.',
      },
    },
  },
};

