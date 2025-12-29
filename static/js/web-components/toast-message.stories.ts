/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-type-assertion -- Storybook stories use dynamic element creation with any types */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import { AugmentedError, ErrorKind } from './augment-error-service.js';
import './toast-message.js';

const meta: Meta = {
  title: 'Components/ToastMessage',
  component: 'toast-message',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A compact notification component styled to match the system-info design language.

**Key Visual Features:**
- Translucent dark grey background (#2d2d2d) matching system-info component
- Opacity transition from 0.2 to 0.9 on hover for subtle prominence
- Compact typography with monospace fonts for technical content
- System-info color palette and border styling
- Smooth transitions and micro-interactions
- Mobile-responsive design

**Usage:**
\`\`\`typescript
import { showToast } from './toast-message.js';
showToast('Operation completed', 'success', 3);
\`\`\`
        `,
      },
    },
  },
  argTypes: {
    message: { control: 'text' },
    type: {
      control: { type: 'select' },
      options: ['success', 'error', 'warning', 'info']
    },
    visible: { control: 'boolean' },
    autoClose: { control: 'boolean' },
    timeoutSeconds: { control: 'number' },
  },
};

export default meta;
type Story = StoryObj;

export const CompactSuccess: Story = {
  render: () => {
    const el = document.createElement('toast-message') as any;
    el.message = 'Index rebuilt successfully';
    el.type = 'success';
    el.visible = true;
    el.autoClose = false;
    el.timeoutSeconds = 3;
    
    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 200px; position: relative;">
        <h3>Compact Success Toast</h3>
        <p>System-info styled success notification with monospace typography</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Notice the compact design, monospace font, and system-info color palette.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the new compact styling for success messages. Features monospace typography, tight spacing, and system-info color scheme.',
      },
    },
  },
};

export const CompactError: Story = {
  render: () => {
    const el = document.createElement('toast-message') as any;
    el.message = 'Connection timeout: 30s';
    el.type = 'error';
    el.visible = true;
    el.autoClose = false;
    el.timeoutSeconds = 0;
    
    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 200px; position: relative;">
        <h3>Compact Error Toast</h3>
        <p>Error notifications with technical monospace styling</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Error toasts don't auto-close by default, allowing users time to read technical details.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows error styling with technical monospace font. Error toasts persist by default for user review.',
      },
    },
  },
};

export const CompactWarning: Story = {
  render: () => {
    const el = document.createElement('toast-message') as any;
    el.message = 'Queue: 127 items pending';
    el.type = 'warning';
    el.visible = true;
    el.autoClose = true;
    el.timeoutSeconds = 5;
    
    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 200px; position: relative;">
        <h3>Compact Warning Toast</h3>
        <p>System status warnings with compact design</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Warning messages use system-info yellow (#ffc107) and can auto-close.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Warning toast with system-info styling. Uses compact layout and monospace typography for technical information.',
      },
    },
  },
};

export const CompactInfo: Story = {
  render: () => {
    const el = document.createElement('toast-message') as any;
    el.message = 'Ctrl+K: Search, Ctrl+E: Edit';
    el.type = 'info';
    el.visible = true;
    el.autoClose = true;
    el.timeoutSeconds = 4;
    
    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 200px; position: relative;">
        <h3>Compact Info Toast</h3>
        <p>Keyboard shortcuts and helpful tips</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Info messages use muted colors and monospace font for keyboard shortcuts.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Info toast styled for technical content like keyboard shortcuts. Uses system-info muted color palette.',
      },
    },
  },
};

export const SystemInfoErrorDisplay: Story = {
  render: () => {
    // Create a sample AugmentedError styled for system-info aesthetic
    const originalError = new window.Error('Index rebuild failed: Insufficient disk space (2.1GB required, 1.3GB available)');
    originalError.stack = `Error: Index rebuild failed: Insufficient disk space
    at IndexBuilder.rebuild (/static/js/indexing/builder.js:127:18)
    at IndexManager.refreshAll (/static/js/indexing/manager.js:89:24)
    at SystemInfoIndexing.handleRebuild (/static/js/components/system-info.js:45:12)`;

    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.SERVER,
      'server',
      'rebuilding search index'
    );

    const el = document.createElement('toast-message') as any;
    el.type = 'error';
    el.visible = true;
    el.autoClose = false;
    el.augmentedError = augmentedError;

    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 300px; position: relative;">
        <h3>System Error with Details</h3>
        <p>AugmentedError integration with compact system-info styling</p>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Technical errors embed the error-display component. Click error details to expand stack trace.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows AugmentedError integration with the new compact styling. Error display components maintain their functionality within the system-info design language.',
      },
    },
  },
};

export const InteractiveCloseDemo: Story = {
  render: () => {
    const el = document.createElement('toast-message') as any;
    el.message = 'Rate: 42.3/s Queue: 89';
    el.type = 'info';
    el.visible = true;
    el.autoClose = false;
    el.timeoutSeconds = 0;

    // Add event listeners for action logging
    el.addEventListener('click', action('toast-clicked'));
    el.addEventListener('show', action('toast-shown'));
    el.addEventListener('hide', action('toast-hidden'));

    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 250px; position: relative;">
        <h3>Interactive Close Button Test</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click the small X button in the top-right corner</li>
          <li>Try clicking elsewhere on the toast (should also dismiss)</li>
          <li>Notice the smooth scale animation on button interaction</li>
          <li>Observe the compact close button design</li>
        </ul>
        ${el}
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive test for the new compact close button. Features system-info styling with smooth micro-animations. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

export const ResponsiveDesignDemo: Story = {
  render: () => {
    return html`
      <div style="padding: 20px; background: #f0f8ff;">
        <h3>Responsive Design Test</h3>
        <p>Resize your browser window to see responsive behavior:</p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li><strong>Desktop:</strong> Fixed position top-right, max-width 400px</li>
          <li><strong>Mobile:</strong> Full-width with reduced padding and font sizes</li>
          <li><strong>Typography:</strong> Scales from 11px to 10px on mobile</li>
        </ul>
        
        ${(() => {
          const success = document.createElement('toast-message') as any;
          success.message = 'Build completed: 1.2s';
          success.type = 'success';
          success.visible = true;
          success.autoClose = false;
          success.style.position = 'relative';
          success.style.display = 'block';
          success.style.margin = '10px 0';
          success.style.transform = 'none';
          success.style.opacity = '1';
          return success;
        })()}
        
        ${(() => {
          const warning = document.createElement('toast-message') as any;
          warning.message = 'Queue depth: 156 items';
          warning.type = 'warning';
          warning.visible = true;
          warning.autoClose = false;
          warning.style.position = 'relative';
          warning.style.display = 'block';
          warning.style.margin = '10px 0';
          warning.style.transform = 'none';
          warning.style.opacity = '1';
          return warning;
        })()}
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Resize browser window to test responsive breakpoints at 768px.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates responsive design behavior. Toast components adapt their sizing and typography for mobile devices while maintaining the system-info aesthetic.',
      },
    },
  },
};

export const SystemInfoIntegrationDemo: Story = {
  render: () => {
    // Simulate multiple system-style notifications
    const notifications = [
      { message: 'Index: 450/500 complete', type: 'info', delay: 0 },
      { message: 'Rate: 25.3/s', type: 'success', delay: 500 },
      { message: 'Queue: 127 pending', type: 'warning', delay: 1000 },
      { message: 'Connection timeout', type: 'error', delay: 1500 },
    ];

    return html`
      <div style="padding: 20px; background: #f0f8ff; min-height: 300px; position: relative;">
        <h3>System-Info Integration Demo</h3>
        <p>Multiple toasts demonstrating system-info design consistency:</p>
        <button @click=${() => {
          notifications.forEach(({ message, type, delay }) => {
            setTimeout(() => {
              const el = document.createElement('toast-message') as any;
              el.message = message;
              el.type = type;
              el.visible = false;
              el.autoClose = type !== 'error';
              el.timeoutSeconds = 3;
              
              document.body.appendChild(el);
              requestAnimationFrame(() => el.show());
            }, delay);
          });
        }}>
          Trigger System Notifications
        </button>
        
        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>✅ Compact monospace typography matching system-info</li>
            <li>✅ Consistent color palette and spacing</li>
            <li>✅ Smooth animations and transitions</li>
            <li>✅ Technical content formatting</li>
            <li>⚠️ Error toasts persist for user review</li>
          </ul>
        </div>
        
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Click the button to see multiple system-style notifications with staggered timing.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive demo showing system-info design integration. Multiple toast types demonstrate the cohesive visual language and technical typography.',
      },
    },
  },
};
