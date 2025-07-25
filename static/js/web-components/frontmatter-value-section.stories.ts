import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-value-section.js';

const meta: Meta = {
  title: 'Components/FrontmatterValueSection',
  tags: ['autodocs'],
  component: 'frontmatter-value-section',
  argTypes: {
    fields: {
      control: 'object',
      description: 'Object containing key-value pairs',
    },
    disabled: {
      control: 'boolean',
      description: 'Whether the section is readonly',
    },
    isRoot: {
      control: 'boolean',
      description: 'Whether this is the root section (affects styling)',
    },
    title: {
      control: 'text',
      description: 'Title for the section',
    },
  },
};

export default meta;
type Story = StoryObj;

export const NestedSection: Story = {
  args: {
    fields: {
      title: 'Sample Page Title',
      author: 'John Doe',
      date: '2024-01-15',
      tags: ['javascript', 'web-components'],
    },
    disabled: false,
    isRoot: false,
    title: 'Page Metadata',
  },
  render: (args) => html`
    <frontmatter-value-section 
      .fields="${args.fields}"
      .disabled="${args.disabled}"
      .isRoot="${args.isRoot}"
      .title="${args.title}">
    </frontmatter-value-section>
  `,
};

export const RootSection: Story = {
  args: {
    fields: {
      title: 'Document Title',
      author: 'Jane Smith',
      category: 'documentation',
      published: true,
    },
    disabled: false,
    isRoot: true,
    title: 'Frontmatter',
  },
  render: (args) => html`
    <frontmatter-value-section 
      .fields="${args.fields}"
      .disabled="${args.disabled}"
      .isRoot="${args.isRoot}"
      .title="${args.title}">
    </frontmatter-value-section>
  `,
};

export const Empty: Story = {
  args: {
    fields: {},
    disabled: false,
    isRoot: false,
    title: 'Empty Section',
  },
  render: (args) => html`
    <frontmatter-value-section 
      .fields="${args.fields}"
      .disabled="${args.disabled}"
      .isRoot="${args.isRoot}"
      .title="${args.title}">
    </frontmatter-value-section>
  `,
};

export const Disabled: Story = {
  args: {
    fields: {
      readonly_field: 'This cannot be modified',
      another_field: 'Also readonly',
      nested_tags: ['tag1', 'tag2'],
    },
    disabled: true,
    isRoot: false,
    title: 'Read-only Section',
  },
  render: (args) => html`
    <frontmatter-value-section 
      .fields="${args.fields}"
      .disabled="${args.disabled}"
      .isRoot="${args.isRoot}"
      .title="${args.title}">
    </frontmatter-value-section>
  `,
};