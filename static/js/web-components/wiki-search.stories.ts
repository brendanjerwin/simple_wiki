import type { Meta, StoryObj } from '@storybook/web-components';
import { html } from 'lit';
import './wiki-search.js';

const meta: Meta = {
  title: 'Components/WikiSearch',
  component: 'wiki-search',
  parameters: {
    layout: 'centered',
  },
  argTypes: {},
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <div style="width: 400px; padding: 20px;">
      <wiki-search></wiki-search>
    </div>
  `,
};

export const WithContext: Story = {
  render: () => html`
    <div style="width: 600px; padding: 20px; background: #f8f9fa;">
      <h2>Wiki Navigation</h2>
      <p>Search for pages in the wiki:</p>
      <wiki-search></wiki-search>
      <p style="margin-top: 20px; font-size: 0.9em; color: #666;">
        Tip: Use Ctrl+K or Cmd+K to quickly focus the search box
      </p>
    </div>
  `,
};