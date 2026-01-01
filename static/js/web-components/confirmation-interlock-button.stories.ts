import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import './confirmation-interlock-button.js';
import type { ConfirmationInterlockButton } from './confirmation-interlock-button.js';

const meta: Meta = {
  title: 'Components/ConfirmationInterlockButton',
  tags: ['autodocs'],
  component: 'confirmation-interlock-button',
  parameters: {
    docs: {
      description: {
        component: `
A reusable inline confirmation button that transforms in place from a simple button
to a confirmation prompt with Yes/No options.

**Key Features:**
- Transforms inline without opening a modal
- Configurable labels for trigger, confirmation prompt, and Yes/No buttons
- Auto-disarm timeout (default 5 seconds)
- Disabled state support
- Events: \`confirmed\` and \`cancelled\`

**Use Cases:**
- Confirming destructive actions inline
- Template/settings changes that require confirmation
- Any action where a modal would be disruptive but confirmation is needed

**Usage:**
\`\`\`html
<confirmation-interlock-button
  label="Change"
  confirmLabel="Clear frontmatter?"
  yesLabel="Yes"
  noLabel="No"
  @confirmed=\${this._handleConfirmed}
></confirmation-interlock-button>
\`\`\`
        `,
      },
    },
  },
  argTypes: {
    label: {
      control: 'text',
      description: 'Label shown on the trigger button in normal state',
    },
    confirmLabel: {
      control: 'text',
      description: 'Prompt text shown in armed state',
    },
    yesLabel: {
      control: 'text',
      description: 'Label for the confirm/yes button',
    },
    noLabel: {
      control: 'text',
      description: 'Label for the cancel/no button',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the button is disabled',
    },
    disarmTimeoutMs: {
      control: 'number',
      description: 'Auto-disarm timeout in milliseconds (0 to disable)',
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>Default Confirmation Button</h3>
      <p>Click the button to see the inline confirmation prompt.</p>
      <confirmation-interlock-button
        @confirmed=${action('confirmed')}
        @cancelled=${action('cancelled')}
      ></confirmation-interlock-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Default button with standard labels. Click to arm, then choose Yes or No.',
      },
    },
  },
};

export const CustomLabels: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>Custom Labels</h3>
      <p>Button with custom labels for a template change scenario.</p>
      <confirmation-interlock-button
        label="Change Template"
        confirmLabel="Clear all frontmatter?"
        yesLabel="Clear"
        noLabel="Keep"
        @confirmed=${action('confirmed-clear')}
        @cancelled=${action('cancelled-keep')}
      ></confirmation-interlock-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Customized labels suitable for a template change confirmation.',
      },
    },
  },
};

export const DeleteAction: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>Delete Action</h3>
      <p>Confirmation for a destructive delete action.</p>
      <confirmation-interlock-button
        label="Delete"
        confirmLabel="Delete this item?"
        yesLabel="Delete"
        noLabel="Cancel"
        @confirmed=${action('confirmed-delete')}
        @cancelled=${action('cancelled-delete')}
      ></confirmation-interlock-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Delete confirmation with appropriate labeling.',
      },
    },
  },
};

export const Disabled: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>Disabled State</h3>
      <p>Button is disabled and cannot be armed.</p>
      <confirmation-interlock-button
        label="Reset"
        disabled
        @confirmed=${action('confirmed')}
        @cancelled=${action('cancelled')}
      ></confirmation-interlock-button>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Disabled button that cannot be interacted with.',
      },
    },
  },
};

export const NoAutoDisarm: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>No Auto-Disarm</h3>
      <p>Button stays armed indefinitely until user clicks Yes or No.</p>
      <confirmation-interlock-button
        label="Confirm"
        .disarmTimeoutMs=${0}
        @confirmed=${action('confirmed')}
        @cancelled=${action('cancelled')}
      ></confirmation-interlock-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        With disarmTimeoutMs=0, the button will remain armed until you click Yes or No.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Button with auto-disarm disabled. Remains armed until user action.',
      },
    },
  },
};

export const QuickTimeout: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <h3>Quick Timeout (2 seconds)</h3>
      <p>Button auto-disarms after 2 seconds if no action is taken.</p>
      <confirmation-interlock-button
        label="Quick Action"
        confirmLabel="Act now?"
        .disarmTimeoutMs=${2000}
        @confirmed=${action('confirmed')}
        @cancelled=${action('cancelled')}
      ></confirmation-interlock-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Click the button and wait - it will auto-disarm after 2 seconds.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Button with a quick 2-second auto-disarm timeout.',
      },
    },
  },
};

export const InFormContext: Story = {
  render: () => html`
    <div style="padding: 20px; max-width: 500px;">
      <h3>In Form Context</h3>
      <p>Shows how the button integrates within a form layout.</p>

      <div style="border: 1px solid #ddd; border-radius: 8px; padding: 16px; background: white;">
        <div style="margin-bottom: 16px;">
          <label style="display: block; font-weight: 500; margin-bottom: 6px;">Template</label>
          <div style="display: flex; align-items: center; gap: 12px;">
            <select style="flex: 1; padding: 8px; border: 1px solid #ddd; border-radius: 4px;" disabled>
              <option>article-template</option>
            </select>
            <confirmation-interlock-button
              label="Change"
              confirmLabel="Clear frontmatter?"
              @confirmed=${action('template-change-confirmed')}
              @cancelled=${action('template-change-cancelled')}
            ></confirmation-interlock-button>
          </div>
          <p style="margin-top: 4px; font-size: 12px; color: #666;">
            Changing template will clear current frontmatter values.
          </p>
        </div>

        <div style="margin-bottom: 16px;">
          <label style="display: block; font-weight: 500; margin-bottom: 6px;">Frontmatter</label>
          <div style="background: #f5f5f5; padding: 12px; border-radius: 4px; font-family: monospace; font-size: 13px;">
            author = "John Doe"<br>
            date = "2024-01-15"<br>
            tags = ["blog", "tech"]
          </div>
        </div>
      </div>

      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates the button in a realistic form context, like the insert-new-page dialog.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    return html`
      <div style="padding: 20px;">
        <h3>Interactive Testing</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click "Arm" to arm the button programmatically</li>
          <li>Click "Disarm" to disarm it</li>
          <li>Test the button's click behavior directly</li>
          <li>Observe auto-disarm after 5 seconds</li>
        </ul>

        <div style="display: flex; gap: 12px; align-items: center; margin: 20px 0;">
          <confirmation-interlock-button
            id="test-button"
            label="Test Me"
            @confirmed=${action('confirmed-test')}
            @cancelled=${action('cancelled-test')}
          ></confirmation-interlock-button>

          <button @click=${() => {
            const btn = document.querySelector('#test-button') as ConfirmationInterlockButton;
            btn?.arm();
          }} style="padding: 8px 16px;">Arm</button>

          <button @click=${() => {
            const btn = document.querySelector('#test-button') as ConfirmationInterlockButton;
            btn?.disarm();
          }} style="padding: 8px 16px;">Disarm</button>
        </div>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>Normal state shows single button with label</li>
            <li>Armed state shows prompt + Yes/No buttons</li>
            <li>Yes dispatches 'confirmed' event and disarms</li>
            <li>No dispatches 'cancelled' event and disarms</li>
            <li>Auto-disarms after 5 seconds (default)</li>
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
        story: 'Interactive testing with programmatic arm/disarm controls. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};
