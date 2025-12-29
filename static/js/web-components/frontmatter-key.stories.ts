import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-key.js';

const meta: Meta = {
  title: 'Components/FrontmatterKey',
  tags: ['autodocs'],
  component: 'frontmatter-key',
  argTypes: {
    key: {
      control: 'text',
      description: 'The key name to display',
    },
    editable: {
      control: 'boolean',
      description: 'Whether the key can be edited',
    },
    placeholder: {
      control: 'text',
      description: 'Placeholder text for editable mode',
    },
  },
};

export default meta;
type Story = StoryObj;

export const ReadOnly: Story = {
  args: {
    key: 'title',
    editable: false,
  },
  render: (args) => html`
    <frontmatter-key
      .key="${args['key']}"
      .editable="${args['editable']}">
    </frontmatter-key>
  `,
};

export const Editable: Story = {
  args: {
    key: 'author',
    editable: true,
    placeholder: 'Enter key name...',
  },
  render: (args) => html`
    <frontmatter-key
      .key="${args['key']}"
      .editable="${args['editable']}"
      .placeholder="${args['placeholder']}">
    </frontmatter-key>
  `,
};

export const Empty: Story = {
  args: {
    key: '',
    editable: true,
    placeholder: 'Enter key name...',
  },
  render: (args) => html`
    <frontmatter-key
      .key="${args['key']}"
      .editable="${args['editable']}"
      .placeholder="${args['placeholder']}">
    </frontmatter-key>
  `,
};