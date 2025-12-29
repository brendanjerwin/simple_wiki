import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-value-string.js';

const meta: Meta = {
  title: 'Components/FrontmatterValueString',
  tags: ['autodocs'],
  component: 'frontmatter-value-string',
  argTypes: {
    value: {
      control: 'text',
      description: 'The string value to display and edit',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the input is disabled',
    },
    placeholder: {
      control: 'text',
      description: 'Placeholder text for empty input',
    },
  },
};

export default meta;
type Story = StoryObj;

export const WithValue: Story = {
  args: {
    value: 'Sample string value',
    disabled: false,
    placeholder: 'Enter value...',
  },
  render: (args) => html`
    <frontmatter-value-string
      .value="${args['value']}"
      .disabled="${args['disabled']}"
      .placeholder="${args['placeholder']}">
    </frontmatter-value-string>
  `,
};

export const Empty: Story = {
  args: {
    value: '',
    disabled: false,
    placeholder: 'Enter a value here...',
  },
  render: (args) => html`
    <frontmatter-value-string
      .value="${args['value']}"
      .disabled="${args['disabled']}"
      .placeholder="${args['placeholder']}">
    </frontmatter-value-string>
  `,
};

export const Disabled: Story = {
  args: {
    value: 'This value cannot be changed',
    disabled: true,
    placeholder: 'Enter value...',
  },
  render: (args) => html`
    <frontmatter-value-string
      .value="${args['value']}"
      .disabled="${args['disabled']}"
      .placeholder="${args['placeholder']}">
    </frontmatter-value-string>
  `,
};