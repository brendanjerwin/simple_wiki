import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './wiki-search-results.js';
import type { SearchResultWithHTML } from '../services/search-client.js';

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

const mockResults: SearchResultWithHTML[] = [
  {
    identifier: 'getting-started',
    title: 'Getting Started with Simple Wiki',
    fragment: 'Welcome to Simple Wiki! This guide will help you get started with creating and editing pages.',
    highlights: [],
    fragmentHTML: 'Welcome to <mark>Simple</mark> <mark>Wiki</mark>! This guide will help you get started with creating and editing pages.'
  },
  {
    identifier: 'advanced-features', 
    title: 'Advanced Features',
    fragment: 'Learn about frontmatter, search functionality, and other advanced features.',
    highlights: [],
    fragmentHTML: 'Learn about <mark>frontmatter</mark>, search functionality, and other <mark>advanced</mark> features.'
  },
  {
    identifier: 'troubleshooting',
    title: 'Troubleshooting Common Issues',
    fragment: 'Solutions to common problems you might encounter while using the wiki.',
    highlights: [],
    fragmentHTML: 'Solutions to <mark>common</mark> problems you might encounter while using the wiki.'
  }
];

const longContentResults: SearchResultWithHTML[] = [
  {
    identifier: 'long-article',
    title: 'Very Long Article with Multiple Matches',
    fragment: 'This is a very long fragment that demonstrates how the search results handle longer content with multiple highlighted terms and line breaks.',
    highlights: [],
    fragmentHTML: 'This is a very <mark>long</mark> fragment that demonstrates how the <mark>search</mark> results handle longer content with multiple <mark>highlighted</mark> terms and line breaks.<br>It can span multiple lines and still display properly in the popover.'
  },
  {
    identifier: 'special-chars',
    title: 'Article with Special Characters & HTML',
    fragment: 'Content with <script> tags and & special characters that should be properly escaped.',
    highlights: [],
    fragmentHTML: 'Content with &lt;script&gt; tags and &amp; <mark>special</mark> characters that should be properly escaped.'
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
      .open="${args.open}"
      @search-results-closed="${action('search-results-closed')}">
    </wiki-search-results>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Basic search results popover in the open state. Click the X button or outside the popover to close it. Open browser dev tools to see the action logs.'
      }
    }
  }
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
  parameters: {
    docs: {
      description: {
        story: 'Search results in closed state. The component exists but the popover is hidden.'
      }
    }
  }
};

export const Empty: Story = {
  args: {
    results: [],
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}"
      @search-results-closed="${action('search-results-closed')}">
    </wiki-search-results>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Empty search results state. No results are displayed, but the popover structure is still visible.'
      }
    }
  }
};

export const LongContent: Story = {
  args: {
    results: longContentResults,
    open: true,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}"
      @search-results-closed="${action('search-results-closed')}">
    </wiki-search-results>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Search results with longer content and special characters to test overflow handling and HTML escaping.'
      }
    }
  }
};

export const InteractiveTesting: Story = {
  args: {
    results: mockResults,
    open: true,
  },
  render: (args) => html`
    <div style="height: 400px; display: flex; flex-direction: column; align-items: center; justify-content: center;">
      <wiki-search-results 
        .results="${args.results}"
        .open="${args.open}"
        @search-results-closed="${action('search-results-closed')}">
      </wiki-search-results>
      <p style="margin-top: 20px; text-align: center; color: #666;">
        Click on search result links, close button, or outside the popover to test interactions.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive testing story for manual interaction with the search results component. Open browser dev tools to see action logs for close events.'
      }
    }
  }
};