import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './wiki-search.js';

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

export const WithContext: Story = {
  render: () => html`
    <div style="width: 600px; padding: 20px; background: #f8f9fa;">
      <h2>Wiki Navigation</h2>
      <p>Search for pages in the wiki:</p>
      <wiki-search
        @input=${action('search-input')}
        @submit=${action('search-submitted')}
        @focus=${action('search-focused')}
        @keydown=${action('keydown-event')}>
      </wiki-search>
      <p style="margin-top: 20px; font-size: 0.9em; color: #666;">
        Tip: Use Ctrl+K or Cmd+K to quickly focus the search box
      </p>
    </div>
  `,
};

// Interactive story to test keyboard shortcuts
export const KeyboardShortcuts: Story = {
  render: () => html`
    <div style="width: 600px; padding: 20px; background: #f0f8ff; border: 1px solid #ddd; border-radius: 8px;">
      <h3 style="margin-top: 0;">Keyboard Shortcuts Test</h3>
      <p>Try the following keyboard shortcuts:</p>
      <ul style="margin-bottom: 20px;">
        <li><strong>Ctrl+K</strong> (or <strong>Cmd+K</strong> on Mac) - Focus the search input</li>
        <li><strong>Enter</strong> - Submit search</li>
        <li><strong>Escape</strong> - Clear and blur search</li>
      </ul>
      <wiki-search
        @input=${action('search-input')}
        @submit=${action('search-submitted')}
        @focus=${action('search-focused')}
        @keydown=${action('keydown-event')}>
      </wiki-search>
      <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
        Watch the browser console (F12) to see triggered events logged.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'This story demonstrates keyboard shortcuts and interaction testing. Try using Ctrl+K to focus, typing search terms, pressing Enter to submit, and Escape to clear. Watch the browser console for event tracking.',
      },
    },
  },
};