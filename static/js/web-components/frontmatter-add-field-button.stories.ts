import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import './frontmatter-add-field-button.js';

// Custom action logger for Storybook
const action = (name: string) => (event: Event) => {
  console.log(`🎬 Action: ${name}`, {
    type: event.type,
    target: event.target,
    detail: (event as CustomEvent).detail,
    timestamp: new Date().toISOString()
  });
};

const meta: Meta = {
  title: 'Components/FrontmatterAddFieldButton',
  component: 'frontmatter-add-field-button',
  parameters: {
    layout: 'centered',
  },
  argTypes: {
    disabled: { control: 'boolean' },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  args: {
    disabled: false,
  },
  render: (args) => html`
    <div style="padding: 20px;">
      <frontmatter-add-field-button 
        ?disabled=${args.disabled}
        @add-field=${action('add-field-event')}
        @click=${action('button-clicked')}>
      </frontmatter-add-field-button>
    </div>
  `,
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
  render: (args) => html`
    <div style="padding: 20px;">
      <frontmatter-add-field-button 
        ?disabled=${args.disabled}
        @add-field=${action('add-field-event')}
        @click=${action('button-clicked')}>
      </frontmatter-add-field-button>
    </div>
  `,
};

export const InContext: Story = {
  args: {
    disabled: false,
  },
  render: (args) => html`
    <div style="padding: 20px; background: #f8f9fa; border: 1px solid #e9ecef; border-radius: 4px;">
      <h3>Frontmatter Editor</h3>
      <div style="margin-bottom: 10px;">
        <label style="display: block; margin-bottom: 5px;">title</label>
        <input type="text" value="My Wiki Page" style="width: 100%; padding: 5px;">
      </div>
      <div style="margin-bottom: 10px;">
        <label style="display: block; margin-bottom: 5px;">tags</label>
        <input type="text" value="tutorial, howto" style="width: 100%; padding: 5px;">
      </div>
      <div style="margin-top: 15px;">
        <frontmatter-add-field-button 
          ?disabled=${args.disabled}
          @add-field=${action('add-field-event')}
          @click=${action('button-clicked')}>
        </frontmatter-add-field-button>
      </div>
    </div>
  `,
};

// Interactive dropdown testing story
export const InteractiveDropdown: Story = {
  args: {
    disabled: false,
  },
  render: (args) => html`
    <div style="padding: 40px; background: #f0f8ff; border: 1px solid #ddd; border-radius: 8px; min-height: 200px;">
      <h3 style="margin-top: 0;">Dropdown Interaction Test</h3>
      <p>Click the button to open the dropdown, then select different field types:</p>
      <ul style="margin-bottom: 20px;">
        <li><strong>Add Field</strong> - Creates a simple string field</li>
        <li><strong>Add Array</strong> - Creates an array field</li>
        <li><strong>Add Section</strong> - Creates a nested object field</li>
      </ul>
      <frontmatter-add-field-button 
        ?disabled=${args.disabled}
        @add-field=${action('add-field-event')}
        @click=${action('button-clicked')}>
      </frontmatter-add-field-button>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Watch the browser console (F12) to see the events and their payload data.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'This story demonstrates dropdown interactions. Click the button to open the dropdown menu and select different field types. Each selection triggers an add-field event with specific type information. Open the browser developer tools console to see the action logs.',
      },
    },
  },
};