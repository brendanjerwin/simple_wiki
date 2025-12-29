import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './wiki-search-results.js';
import { SearchResultSchema, HighlightSpanSchema, InventoryContextSchema, ContainerPathElementSchema } from '../gen/api/v1/search_pb.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';
import { create } from '@bufbuild/protobuf';

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

// Create highlight spans for the mock data using protobuf create function
function createHighlight(start: number, end: number) {
  return create(HighlightSpanSchema, { start, end });
}

// Create a search result using protobuf create function
function createSearchResult(data: {
  identifier: string;
  title: string;
  fragment: string;
  highlights: ReturnType<typeof createHighlight>[];
}): SearchResult {
  return create(SearchResultSchema, data);
}

// Create a container path element using protobuf create function
function createContainerPathElement(data: { identifier: string; title: string; depth: number }) {
  return create(ContainerPathElementSchema, data);
}

// Create an inventory context using protobuf create function
function createInventoryContext(data: {
  isInventoryRelated: boolean;
  path: ReturnType<typeof createContainerPathElement>[];
}) {
  return create(InventoryContextSchema, data);
}

// Create a search result with inventory context using protobuf create function
function createInventorySearchResult(data: {
  identifier: string;
  title: string;
  fragment: string;
  highlights: ReturnType<typeof createHighlight>[];
  inventoryContext: {
    isInventoryRelated: boolean;
    path: Array<{ identifier: string; title: string; depth: number }>;
  };
}): SearchResult {
  return create(SearchResultSchema, {
    identifier: data.identifier,
    title: data.title,
    fragment: data.fragment,
    highlights: data.highlights,
    inventoryContext: createInventoryContext({
      isInventoryRelated: data.inventoryContext.isInventoryRelated,
      path: data.inventoryContext.path.map(p => createContainerPathElement(p)),
    }),
  });
}

const mockResults: SearchResult[] = [
  createSearchResult({
    identifier: 'getting-started',
    title: 'Getting Started with Simple Wiki',
    fragment: 'Welcome to Simple Wiki! This guide will help you get started with creating and editing pages.',
    highlights: [
      createHighlight(11, 17), // "Simple"
      createHighlight(18, 22),  // "Wiki"
    ]
  }),
  createSearchResult({
    identifier: 'advanced-features',
    title: 'Advanced Features',
    fragment: 'Learn about frontmatter, search functionality, and other advanced features.',
    highlights: [
      createHighlight(12, 23), // "frontmatter"
      createHighlight(58, 66),  // "advanced"
    ]
  }),
  createSearchResult({
    identifier: 'troubleshooting',
    title: 'Troubleshooting Common Issues',
    fragment: 'Solutions to common problems you might encounter while using the wiki.',
    highlights: [
      createHighlight(13, 19), // "common"
    ]
  })
];

const longContentResults: SearchResult[] = [
  createSearchResult({
    identifier: 'long-article',
    title: 'Very Long Article with Multiple Matches',
    fragment: 'This is a very long fragment that demonstrates how the search results handle longer content with multiple highlighted terms and line breaks.\nIt can span multiple lines and still display properly in the popover.',
    highlights: [
      createHighlight(15, 19), // "long"
      createHighlight(66, 72),  // "search"
      createHighlight(112, 123), // "highlighted"
    ]
  }),
  createSearchResult({
    identifier: 'special-chars',
    title: 'Article with Special Characters & HTML',
    fragment: 'Content with <script> tags and & special characters that should be properly escaped.',
    highlights: [
      createHighlight(39, 46), // "special"
    ]
  })
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
    results: [createSearchResult({
      identifier: 'no-highlights',
      title: 'Article Without Highlights',
      fragment: 'This fragment has no highlighted terms and should display as plain text.',
      highlights: []
    })],
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

export const WithInventoryContainer: Story = {
  args: {
    results: [
      createInventorySearchResult({
        identifier: 'screwdriver',
        title: 'Phillips Head Screwdriver',
        fragment: 'A versatile screwdriver with a #2 Phillips head, perfect for most household tasks.',
        highlights: [
          createHighlight(14, 25), // "screwdriver"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'garage', title: 'Main Garage', depth: 1 },
            { identifier: 'toolbox', title: 'My Toolbox', depth: 2 }
          ]
        }
      }),
      createInventorySearchResult({
        identifier: 'hammer',
        title: 'Claw Hammer',
        fragment: 'Standard 16oz claw hammer for general construction and demolition work.',
        highlights: [
          createHighlight(18, 24), // "hammer"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'garage', title: 'Main Garage', depth: 1 },
            { identifier: 'toolbox', title: 'My Toolbox', depth: 2 }
          ]
        }
      }),
      createInventorySearchResult({
        identifier: 'wrench',
        title: 'Adjustable Wrench',
        fragment: '10-inch adjustable wrench for nuts and bolts.',
        highlights: [
          createHighlight(27, 33), // "wrench"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'garage_cabinet', title: '', depth: 0 }
          ]
        }
      }),
      createInventorySearchResult({
        identifier: 'power_drill',
        title: 'Cordless Power Drill',
        fragment: '18V cordless drill with battery and charger.',
        highlights: [
          createHighlight(19, 24), // "drill"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'workshop_shed', title: '', depth: 1 },
            { identifier: 'red_case', title: 'Red Tool Case', depth: 2 }
          ]
        }
      }),
      createInventorySearchResult({
        identifier: 'hex_key',
        title: 'Allen Hex Key Set',
        fragment: 'Metric hex key set in organized case.',
        highlights: [
          createHighlight(7, 10), // "hex"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'building', title: 'Main Building', depth: 0 },
            { identifier: 'floor2', title: 'Second Floor', depth: 1 },
            { identifier: 'storage_room', title: 'Storage Room', depth: 2 },
            { identifier: 'shelf_a', title: 'Metal Shelf A', depth: 3 },
            { identifier: 'big_box', title: 'Large Storage Box', depth: 4 },
            { identifier: 'small_box', title: 'Small Organizer Box', depth: 5 }
          ]
        }
      })
    ],
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
        story: 'Search results showing items with inventory containers. The "In" section displays the full container path with each level clickable. Demonstrates various scenarios:\n- Full path with all titles (screwdriver & hammer)\n- Single level with no title, falls back to identifier (wrench)\n- **Mixed scenario**: Path with some titles and some identifiers (drill shows "My House › workshop_shed › Red Tool Case")\n- **Long path truncation**: Paths with more than 4 levels show "..." for early items, keeping the deepest (most useful) levels visible (hex key shows "... › Metal Shelf A › Large Storage Box › Small Organizer Box")'
      }
    }
  }
};

export const InventoryFilterWarning: Story = {
  args: {
    results: [
      createInventorySearchResult({
        identifier: 'screwdriver',
        title: 'Phillips Head Screwdriver',
        fragment: 'A versatile screwdriver with a #2 Phillips head, perfect for most household tasks.',
        highlights: [
          createHighlight(14, 25), // "screwdriver"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'garage', title: 'Main Garage', depth: 1 },
            { identifier: 'toolbox', title: 'My Toolbox', depth: 2 }
          ]
        }
      }),
      createInventorySearchResult({
        identifier: 'hammer',
        title: 'Claw Hammer',
        fragment: 'Standard 16oz claw hammer for general construction and demolition work.',
        highlights: [
          createHighlight(18, 24), // "hammer"
        ],
        inventoryContext: {
          isInventoryRelated: true,
          path: [
            { identifier: 'house', title: 'My House', depth: 0 },
            { identifier: 'garage', title: 'Main Garage', depth: 1 },
            { identifier: 'toolbox', title: 'My Toolbox', depth: 2 }
          ]
        }
      })
    ],
    open: true,
    inventoryOnly: true,
    totalUnfilteredCount: 7,
  },
  render: (args) => html`
    <wiki-search-results 
      .results="${args.results}"
      .open="${args.open}"
      .inventoryOnly="${args.inventoryOnly}"
      .totalUnfilteredCount="${args.totalUnfilteredCount}"
      @search-results-closed="${action('search-results-closed')}"
      @inventory-filter-changed="${action('inventory-filter-changed')}">
    </wiki-search-results>
  `,
  parameters: {
    docs: {
      description: {
        story: '**Inventory Filter Warning Feature**: Demonstrates the warning message that appears when the "Inventory Only" checkbox is checked and additional results are hidden. In this example, 2 inventory items are shown out of 7 total results, so a warning displays "5 other results not shown (not Inventory Only)" with an amber/yellow background and exclamation icon. This helps users understand that more results are available when the filter is unchecked. Open browser dev tools to see action logs when toggling the checkbox.'
      }
    }
  }
};