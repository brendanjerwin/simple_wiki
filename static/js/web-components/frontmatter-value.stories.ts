import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-value.js';

const meta: Meta = {
  title: 'Components/FrontmatterValue',
  tags: ['autodocs'],
  component: 'frontmatter-value',
  argTypes: {
    value: {
      control: 'object',
      description: 'The value to display (string, array, or object)',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the value is readonly',
    },
    placeholder: {
      control: 'text',
      description: 'Placeholder text for empty values',
    },
  },
};

export default meta;
type Story = StoryObj;

export const StringValue: Story = {
  args: {
    value: 'This is a string value',
    disabled: false,
    placeholder: 'Enter a value...',
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value>
  `,
};

export const ArrayValue: Story = {
  args: {
    value: ['tag1', 'tag2', 'tag3'],
    disabled: false,
    placeholder: 'Add item...',
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value>
  `,
};

export const ObjectValue: Story = {
  args: {
    value: {
      author: 'John Doe',
      date: '2024-01-15',
      category: 'documentation'
    },
    disabled: false,
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}">
    </frontmatter-value>
  `,
};

export const EmptyValue: Story = {
  args: {
    value: null,
    disabled: false,
    placeholder: 'No value set',
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}"
      .placeholder="${args.placeholder}">
    </frontmatter-value>
  `,
};

export const DisabledString: Story = {
  args: {
    value: 'This value cannot be edited',
    disabled: true,
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}">
    </frontmatter-value>
  `,
};

export const NumberValue: Story = {
  args: {
    value: 42,
    disabled: false,
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}">
    </frontmatter-value>
  `,
};

export const BooleanValue: Story = {
  args: {
    value: true,
    disabled: false,
  },
  render: (args) => html`
    <frontmatter-value 
      .value="${args.value}"
      .disabled="${args.disabled}">
    </frontmatter-value>
  `,
};