import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import './wiki-checklist.js';
import type { ChecklistItem } from './wiki-checklist.js';
import { AugmentErrorService } from './augment-error-service.js';

const meta: Meta = {
  title: 'Components/WikiChecklist',
  tags: ['autodocs'],
  component: 'wiki-checklist',
  parameters: {
    layout: 'padded',
    docs: {
      description: {
        component: `
A fully API-driven interactive checklist component backed by the frontmatter gRPC API.

**Features:**
- Check/uncheck items (persisted immediately)
- Inline text editing per item
- Multiple tags per item via \`:tag\` syntax
- Tag filter bar: click a tag pill to filter visible items
- Add new items with optional tags (e.g. "milk :dairy :fridge")
- Remove items
- Drag-and-drop reordering
- Automatic polling every 3 s to stay in sync

**Usage:**
\`\`\`html
<wiki-checklist list-name="grocery_list" page="my-page"></wiki-checklist>
\`\`\`

**Storybook note:** In Storybook, the component has no backend, so stories bypass
the API by setting \`items\` (and optionally \`loading\`, \`error\`, \`filterTag\`)
directly on the element after fixture creation.
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

// ---------------------------------------------------------------------------
// Stories
// ---------------------------------------------------------------------------

export const Default: Story = {
  render: () => {
    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist list-name="my_list"></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Empty checklist with no items. The "No items yet. Add one below!" empty-state message is shown.',
      },
    },
  },
};

export const WithItems: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Milk', checked: false, tags: ['dairy'] },
      { text: 'Eggs', checked: true, tags: ['dairy'] },
      { text: 'Apples', checked: false, tags: ['produce'] },
      { text: 'Bananas', checked: true, tags: ['produce'] },
      { text: 'Sourdough bread', checked: false, tags: ['bakery'] },
      { text: 'Butter', checked: false, tags: [] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .loading=${false}
          .error=${null}
          @change=${action('checkbox-change')}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Mix of checked and unchecked items, some with tags. Checked items show strikethrough and reduced opacity. Tag filter bar appears above the list.',
      },
    },
  },
};

export const AllChecked: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Buy coffee', checked: true, tags: [] },
      { text: 'Read emails', checked: true, tags: [] },
      { text: 'Team stand-up', checked: true, tags: [] },
      { text: 'Code review', checked: true, tags: [] },
      { text: 'Deploy to staging', checked: true, tags: [] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="daily_tasks"
          .items=${items}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'All items are checked. Every row shows the strikethrough + fade style, indicating a completed list.',
      },
    },
  },
};

export const SingleItem: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Pick up dry cleaning', checked: false, tags: [] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="reminders"
          .items=${items}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Minimal checklist with a single item -- useful for verifying layout at minimum size.',
      },
    },
  },
};

export const MultipleTagsPerItem: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Whole milk (2L)', checked: false, tags: ['dairy', 'fridge'] },
      { text: 'Cheddar cheese', checked: true, tags: ['dairy', 'fridge'] },
      { text: 'Greek yogurt', checked: false, tags: ['dairy', 'fridge'] },
      { text: 'Fuji apples', checked: false, tags: ['produce', 'fridge'] },
      { text: 'Baby spinach', checked: false, tags: ['produce', 'fridge'] },
      { text: 'Roma tomatoes', checked: true, tags: ['produce'] },
      { text: 'Sourdough loaf', checked: false, tags: ['bakery'] },
      { text: 'Croissants (x4)', checked: false, tags: ['bakery'] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Grocery list where items can have multiple tags (e.g. "dairy" and "fridge"). ' +
          'The tag filter bar shows all unique tags as clickable pills.',
      },
    },
  },
};

export const FilteredByTag: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Whole milk (2L)', checked: false, tags: ['dairy', 'fridge'] },
      { text: 'Cheddar cheese', checked: true, tags: ['dairy'] },
      { text: 'Fuji apples', checked: false, tags: ['produce'] },
      { text: 'Baby spinach', checked: false, tags: ['produce', 'fridge'] },
      { text: 'Sourdough loaf', checked: false, tags: ['bakery'] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .filterTag=${'dairy'}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Tag filter active on "dairy". Only items tagged with "dairy" are shown. ' +
          'The active pill is highlighted. Click it again to clear the filter.',
      },
    },
  },
};

export const MixedTaggedUntagged: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Write unit tests', checked: false, tags: ['dev'] },
      { text: 'Update README', checked: true, tags: [] },
      { text: 'Fix TypeScript error', checked: false, tags: ['dev'] },
      { text: 'Buy coffee beans', checked: false, tags: [] },
      { text: 'Review PR #42', checked: false, tags: ['dev'] },
      { text: 'Call dentist', checked: true, tags: [] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="mixed_tasks"
          .items=${items}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Some items have tags and some do not. Untagged items show no badges. Filter bar only shows existing tags.',
      },
    },
  },
};

export const LongList: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      // Dairy
      { text: 'Whole milk (2L)', checked: false, tags: ['dairy'] },
      { text: 'Skimmed milk (1L)', checked: true, tags: ['dairy'] },
      { text: 'Cheddar cheese', checked: false, tags: ['dairy'] },
      { text: 'Greek yogurt', checked: false, tags: ['dairy'] },
      { text: 'Butter (250g)', checked: true, tags: ['dairy'] },
      // Produce
      { text: 'Fuji apples (x6)', checked: false, tags: ['produce'] },
      { text: 'Baby spinach', checked: false, tags: ['produce'] },
      { text: 'Roma tomatoes', checked: true, tags: ['produce'] },
      { text: 'Yellow onions', checked: false, tags: ['produce'] },
      { text: 'Garlic (bulb)', checked: false, tags: ['produce'] },
      // Bakery
      { text: 'Sourdough loaf', checked: false, tags: ['bakery'] },
      { text: 'Croissants (x4)', checked: false, tags: ['bakery'] },
      { text: 'Bagels (x6)', checked: true, tags: ['bakery'] },
      // Frozen
      { text: 'Frozen peas (400g)', checked: false, tags: ['frozen'] },
      { text: 'Ice cream (vanilla)', checked: false, tags: ['frozen'] },
      { text: 'Frozen fish fillets', checked: true, tags: ['frozen'] },
      // Pantry
      { text: 'Pasta (500g)', checked: false, tags: ['pantry'] },
      { text: 'Olive oil (500ml)', checked: false, tags: ['pantry'] },
      { text: 'Tinned tomatoes (x4)', checked: true, tags: ['pantry'] },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="big_shop"
          .items=${items}
          .loading=${false}
          .error=${null}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          '19 items spread across 5 tag categories (dairy, produce, bakery, frozen, pantry). ' +
          'Use the tag filter bar to focus on a specific category. ' +
          'Useful for verifying scrolling and that the filter handles many tags cleanly.',
      },
    },
  },
};

export const Loading: Story = {
  render: () => {
    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="shopping_list"
          .loading=${true}
          .error=${null}
          .items=${[]}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Initial loading state shown while the component fetches checklist data from the API. ' +
          'A spinner and "Loading checklist..." message is displayed.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const mockError = AugmentErrorService.augmentError(
      new Error('Network error: Failed to connect to API'),
      'loading checklist'
    );

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="shopping_list"
          .loading=${false}
          .error=${mockError}
          .items=${[]}
        ></wiki-checklist>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Error state when the API call fails. The error-display component is shown with a Retry button.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const groceryItems: ChecklistItem[] = [
      { text: 'Whole milk', checked: false, tags: ['dairy', 'fridge'] },
      { text: 'Eggs (x12)', checked: false, tags: ['dairy'] },
      { text: 'Apples', checked: false, tags: ['produce'] },
      { text: 'Sourdough bread', checked: false, tags: ['bakery'] },
      { text: 'Butter', checked: false, tags: [] },
    ];

    return html`
      <div style="max-width: 700px; padding: 20px;">
        <h3 style="margin-top: 0;">Interactive Checklist Testing</h3>
        <p>
          <strong>Open the browser developer tools console to see the action logs.</strong>
        </p>

        <h4>Test scenarios</h4>
        <ul style="margin-bottom: 20px; padding-left: 20px; font-size: 0.9em; color: #555;">
          <li>Check/uncheck items -- triggers an API save (no backend in Storybook, so a network error is expected)</li>
          <li>Click tag pills in the filter bar to filter items by tag</li>
          <li>Click the active pill again to clear the filter</li>
          <li>Type in the "Add item..." field with :tag syntax and press Enter or click Add</li>
          <li>Click the remove button to remove an item</li>
          <li>Drag items to reorder them</li>
        </ul>

        <wiki-checklist
          list-name="grocery_list"
          .items=${groceryItems}
          .loading=${false}
          .error=${null}
          @click=${action('checklist-click')}
        ></wiki-checklist>

        <div style="margin-top: 24px; padding: 14px; background: #fff3cd; border-radius: 6px; font-size: 0.88em; color: #555;">
          <strong>Note:</strong> This story injects items directly, bypassing the gRPC API.
          Interactions that trigger save/load will result in a network error in Storybook -- this is expected.
          The UI interactions (filtering, drag-and-drop, etc.) work without a backend.
        </div>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Fully interactive story for manual QA. Pre-loaded with grocery items. ' +
          'Test checking items, filtering by tag, adding and removing items, drag-and-drop reordering. ' +
          'Open the browser developer tools console to see the action logs.',
      },
    },
  },
};

// ---------------------------------------------------------------------------
// Decorator-based approach: use a play function to set state post-render
// for stories where Lit property bindings alone are insufficient (e.g. when
// we need to reach into shadow-DOM state).  The stories above use direct
// property bindings which is the preferred pattern.
// ---------------------------------------------------------------------------
