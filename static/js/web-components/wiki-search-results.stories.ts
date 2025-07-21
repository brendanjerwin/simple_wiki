import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './wiki-search-results.js';

const meta: Meta = {
  title: 'Components/WikiSearchResults',
  tags: ['autodocs'],
  component: 'wiki-search-results',
  argTypes: {
    results: {
      control: 'object',
      description: 'Array of search results to display',
    },
    open: {
      control: 'boolean',
      description: 'Whether the search results popover is open',
    },
  },
  parameters: {
    layout: 'centered',
  },
};

export default meta;
type Story = StoryObj;

const mockResults = [
  {
    Identifier: 'getting-started',
    Title: 'Getting Started with Simple Wiki',
    FragmentHTML: 'Welcome to <strong>Simple Wiki</strong>! This guide will help you get started with creating and editing pages.'
  },
  {
    Identifier: 'advanced-features',
    Title: 'Advanced Features',
    FragmentHTML: 'Learn about <em>frontmatter</em>, search functionality, and other advanced features.'
  },
  {
    Identifier: 'troubleshooting',
    Title: 'Troubleshooting Common Issues',
    FragmentHTML: 'Solutions to common problems you might encounter while using the wiki.'
  }
];

export const Open: Story = {
  args: {
    results: mockResults,
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}">
    </wiki-search-results>
  `,
};

export const Closed: Story = {
  args: {
    results: mockResults,
    open: false,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}">
    </wiki-search-results>
  `,
};

export const SingleResult: Story = {
  args: {
    results: [mockResults[0]],
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}">
    </wiki-search-results>
  `,
};

export const NoResults: Story = {
  args: {
    results: [],
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}">
    </wiki-search-results>
  `,
};

export const LongTitles: Story = {
  args: {
    results: [
      {
        Identifier: 'very-long-page-identifier',
        Title: 'This is a Very Long Page Title That Might Wrap to Multiple Lines',
        FragmentHTML: 'This page contains information about how the system handles very long page titles and ensures they display properly in the search results interface.'
      },
      {
        Identifier: 'another-long-title',
        Title: 'Another Extremely Long Page Title That Demonstrates Text Overflow Behavior',
        FragmentHTML: 'Additional content that shows how fragments are displayed when the title is particularly long.'
      }
    ],
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}">
    </wiki-search-results>
  `,
};