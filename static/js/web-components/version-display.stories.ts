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