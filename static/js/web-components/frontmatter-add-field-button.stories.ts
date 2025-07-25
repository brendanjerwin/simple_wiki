import type { Meta, StoryObj } from '@storybook/web-components-vite';
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

export const Enabled: Story = {
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