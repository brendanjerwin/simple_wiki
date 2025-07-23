import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import { expect, userEvent } from '@storybook/test';
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
  play: async ({ canvasElement }) => {
    // Find the button element
    const button = canvasElement.querySelector('frontmatter-add-field-button');
    expect(button).toBeInTheDocument();
    expect(button).toHaveProperty('disabled', false);
    
    // Click the button to test basic interaction
    await userEvent.click(button!);
    
    // Note: The actual dropdown behavior would require more complex shadow DOM interaction
    // This play function demonstrates basic button clicking
  },
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
  play: async ({ canvasElement }) => {
    // Find the button element
    const button = canvasElement.querySelector('frontmatter-add-field-button');
    expect(button).toBeInTheDocument();
    expect(button).toHaveProperty('disabled', false);
    
    // Click the button to open dropdown
    await userEvent.click(button!);
    
    // Wait a moment for potential dropdown to appear
    await new Promise(resolve => setTimeout(resolve, 100));
    
    // Note: Testing actual dropdown menu selection would require
    // accessing shadow DOM elements, which is more complex in Storybook
    // This play function demonstrates the button click interaction
  },
  parameters: {
    docs: {
      description: {
        story: 'This story demonstrates dropdown interactions. The play function automatically clicks the button to trigger the dropdown. For manual testing, click the button to open the dropdown menu and select different field types. Each selection triggers an add-field event with specific type information. Watch both the Interactions panel and browser console (F12) for event tracking.',
      },
    },
  },
};