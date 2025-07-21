import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-value-array.js';

const meta: Meta = {
  title: 'Components/FrontmatterValueArray',
  tags: ['autodocs'],
  component: 'frontmatter-value-array',
  argTypes: {
    values: {
      control: 'object',
      description: 'Array of string values',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the array is readonly',
    },
    placeholder: {
      control: 'text',
      description: 'Placeholder text for new items',
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  args: {
    values: ['tag1', 'tag2', 'tag3'],
    disabled: false,
    placeholder: 'Add new item...',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};

export const SingleItem: Story = {
  args: {
    values: ['single-tag'],
    disabled: false,
    placeholder: 'Add new item...',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};

export const Empty: Story = {
  args: {
    values: [],
    disabled: false,
    placeholder: 'Add first item...',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};

export const Disabled: Story = {
  args: {
    values: ['readonly1', 'readonly2', 'readonly3'],
    disabled: true,
    placeholder: 'Cannot add items',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};

export const ManyItems: Story = {
  args: {
    values: [
      'javascript',
      'typescript',
      'web-components',
      'lit',
      'storybook',
      'frontend',
      'development',
      'ui',
      'components'
    ],
    disabled: false,
    placeholder: 'Add tag...',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};

export const LongItems: Story = {
  args: {
    values: [
      'This is a very long item that might wrap or overflow',
      'Another extremely long item that demonstrates text behavior',
      'Short item'
    ],
    disabled: false,
    placeholder: 'Add item...',
  },
  render: (args) => html`
    <frontmatter-value-array 
      .values="${args.values}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value-array>
  `,
};