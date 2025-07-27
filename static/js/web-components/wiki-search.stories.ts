import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
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
      <wiki-search
        @input=${action('search-input')}
        @submit=${action('search-submitted')}
        @focus=${action('search-focused')}
        @keydown=${action('keydown-event')}>
      </wiki-search>
    </div>
  `,
};
