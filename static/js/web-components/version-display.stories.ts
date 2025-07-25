import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './version-display.js';

const meta: Meta = {
  title: 'Components/VersionDisplay',
  component: 'version-display',
  parameters: {
    layout: 'fullscreen',
  },
  argTypes: {},
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <div style="position: relative; height: 200px; width: 300px; border: 1px solid #ccc;">
      <version-display></version-display>
    </div>
  `,
};

export const WithContext: Story = {
  render: () => html`
    <div style="position: relative; height: 400px; width: 600px; background: #f5f5f5; padding: 20px;">
      <h2>Sample Wiki Page</h2>
      <p>This shows how the version display appears in the bottom-right corner of a page.</p>
      <version-display></version-display>
    </div>
  `,
};