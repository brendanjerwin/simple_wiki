import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './wiki-search-results.js';
import type { SearchResult, HighlightSpan } from '../gen/api/v1/search_pb.js';

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

// Create highlight spans for the mock data
function createHighlight(start: number, end: number): HighlightSpan {
  return { start, end } as HighlightSpan;
}

const mockResults: SearchResult[] = [
  {
    identifier: 'getting-started',
    title: 'Getting Started with Simple Wiki',
    fragment: 'Welcome to Simple Wiki! This guide will help you get started with creating and editing pages.',
    highlights: [
      createHighlight(11, 17), // "Simple"
      createHighlight(18, 22),  // "Wiki"
    ]
  } as SearchResult,
  {
    identifier: 'advanced-features', 
    title: 'Advanced Features',
    fragment: 'Learn about frontmatter, search functionality, and other advanced features.',
    highlights: [
      createHighlight(12, 23), // "frontmatter"
      createHighlight(58, 66),  // "advanced"
    ]
  } as SearchResult,
  {
    identifier: 'troubleshooting',
    title: 'Troubleshooting Common Issues',
    fragment: 'Solutions to common problems you might encounter while using the wiki.',
    highlights: [
      createHighlight(13, 19), // "common"
    ]
  } as SearchResult
];

const longContentResults: SearchResult[] = [
  {
    identifier: 'long-article',
    title: 'Very Long Article with Multiple Matches',
    fragment: 'This is a very long fragment that demonstrates how the search results handle longer content with multiple highlighted terms and line breaks.\nIt can span multiple lines and still display properly in the popover.',
    highlights: [
      createHighlight(15, 19), // "long"
      createHighlight(66, 72),  // "search"
      createHighlight(112, 123), // "highlighted"
    ]
  } as SearchResult,
  {
    identifier: 'special-chars',
    title: 'Article with Special Characters & HTML',
    fragment: 'Content with <script> tags and & special characters that should be properly escaped.',
    highlights: [
      createHighlight(39, 46), // "special"
    ]
  } as SearchResult
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

export const NoHighlights: Story = {
  args: {
    results: [{
      identifier: 'no-highlights',
      title: 'Article Without Highlights',
      fragment: 'This fragment has no highlighted terms and should display as plain text.',
      highlights: []
    } as SearchResult],
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
        story: 'Search results with no highlights to test plain text rendering.'
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
        Component now handles HTML generation from structured data (fragment + highlights).<br>
        Click on search result links, close button, or outside the popover to test interactions.
      </p>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive testing story showing proper separation of concerns. The component generates HTML from structured data (fragment + highlights) instead of receiving pre-generated HTML. Open browser dev tools to see action logs.'
      }
    }
  }
};