import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import './frontmatter-add-field-button.js';

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
        ?disabled=${args.disabled}>
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
        ?disabled=${args.disabled}>
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
          ?disabled=${args.disabled}>
        </frontmatter-add-field-button>
      </div>
    </div>
  `,
};