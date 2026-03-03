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
- Tag badges — click to edit or add a tag
- Grouped view: items organized by tag
- Flat view: all items in order with tag badges
- Add new items with optional tag
- Remove items
- Automatic polling every 3 s to stay in sync

**Usage:**
\`\`\`html
<wiki-checklist list-name="grocery_list" page="my-page"></wiki-checklist>
\`\`\`

**Storybook note:** In Storybook, the component has no backend, so stories bypass
the API by setting \`items\` (and optionally \`groupedView\`, \`loading\`, \`error\`)
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
      { text: 'Milk', checked: false, tag: 'Dairy' },
      { text: 'Eggs', checked: true, tag: 'Dairy' },
      { text: 'Apples', checked: false, tag: 'Produce' },
      { text: 'Bananas', checked: true, tag: 'Produce' },
      { text: 'Sourdough bread', checked: false, tag: 'Bakery' },
      { text: 'Butter', checked: false },
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
          'Mix of checked and unchecked items, some with tags. Checked items show strikethrough and reduced opacity.',
      },
    },
  },
};

export const AllChecked: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Buy coffee', checked: true },
      { text: 'Read emails', checked: true },
      { text: 'Team stand-up', checked: true },
      { text: 'Code review', checked: true },
      { text: 'Deploy to staging', checked: true },
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
      { text: 'Pick up dry cleaning', checked: false },
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
          'Minimal checklist with a single item — useful for verifying layout at minimum size.',
      },
    },
  },
};

export const TaggedGroceryList: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Whole milk (2L)', checked: false, tag: 'Dairy' },
      { text: 'Cheddar cheese', checked: true, tag: 'Dairy' },
      { text: 'Greek yogurt', checked: false, tag: 'Dairy' },
      { text: 'Fuji apples', checked: false, tag: 'Produce' },
      { text: 'Baby spinach', checked: false, tag: 'Produce' },
      { text: 'Roma tomatoes', checked: true, tag: 'Produce' },
      { text: 'Sourdough loaf', checked: false, tag: 'Bakery' },
      { text: 'Croissants (×4)', checked: false, tag: 'Bakery' },
    ];
    const groupOrder = ['Produce', 'Dairy', 'Bakery'];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .groupOrder=${groupOrder}
          .groupedView=${true}
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
          'Realistic grocery list in grouped view (Produce → Dairy → Bakery). ' +
          'The `groupOrder` property controls section ordering.',
      },
    },
  },
};

export const FlatViewWithTags: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Whole milk (2L)', checked: false, tag: 'Dairy' },
      { text: 'Cheddar cheese', checked: true, tag: 'Dairy' },
      { text: 'Greek yogurt', checked: false, tag: 'Dairy' },
      { text: 'Fuji apples', checked: false, tag: 'Produce' },
      { text: 'Baby spinach', checked: false, tag: 'Produce' },
      { text: 'Roma tomatoes', checked: true, tag: 'Produce' },
      { text: 'Sourdough loaf', checked: false, tag: 'Bakery' },
      { text: 'Croissants (×4)', checked: false, tag: 'Bakery' },
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .groupedView=${false}
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
          'Same grocery items as TaggedGroceryList but in flat view. Tags are shown as small clickable badges next to each item.',
      },
    },
  },
};

export const MixedTaggedUntagged: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      { text: 'Write unit tests', checked: false, tag: 'Dev' },
      { text: 'Update README', checked: true },
      { text: 'Fix TypeScript error', checked: false, tag: 'Dev' },
      { text: 'Buy coffee beans', checked: false },
      { text: 'Review PR #42', checked: false, tag: 'Dev' },
      { text: 'Call dentist', checked: true },
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
          'Some items have tags and some do not. Untagged items show a "+tag" badge to invite adding a tag.',
      },
    },
  },
};

export const LongList: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      // Dairy
      { text: 'Whole milk (2L)', checked: false, tag: 'Dairy' },
      { text: 'Skimmed milk (1L)', checked: true, tag: 'Dairy' },
      { text: 'Cheddar cheese', checked: false, tag: 'Dairy' },
      { text: 'Greek yogurt', checked: false, tag: 'Dairy' },
      { text: 'Butter (250g)', checked: true, tag: 'Dairy' },
      // Produce
      { text: 'Fuji apples (×6)', checked: false, tag: 'Produce' },
      { text: 'Baby spinach', checked: false, tag: 'Produce' },
      { text: 'Roma tomatoes', checked: true, tag: 'Produce' },
      { text: 'Yellow onions', checked: false, tag: 'Produce' },
      { text: 'Garlic (bulb)', checked: false, tag: 'Produce' },
      // Bakery
      { text: 'Sourdough loaf', checked: false, tag: 'Bakery' },
      { text: 'Croissants (×4)', checked: false, tag: 'Bakery' },
      { text: 'Bagels (×6)', checked: true, tag: 'Bakery' },
      // Frozen
      { text: 'Frozen peas (400g)', checked: false, tag: 'Frozen' },
      { text: 'Ice cream (vanilla)', checked: false, tag: 'Frozen' },
      { text: 'Frozen fish fillets', checked: true, tag: 'Frozen' },
      // Pantry
      { text: 'Pasta (500g)', checked: false, tag: 'Pantry' },
      { text: 'Olive oil (500ml)', checked: false, tag: 'Pantry' },
      { text: 'Tinned tomatoes (×4)', checked: true, tag: 'Pantry' },
    ];
    const groupOrder = ['Produce', 'Dairy', 'Bakery', 'Frozen', 'Pantry'];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="big_shop"
          .items=${items}
          .groupOrder=${groupOrder}
          .groupedView=${true}
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
          '19 items spread across 5 tag groups (Produce, Dairy, Bakery, Frozen, Pantry). ' +
          'Useful for verifying scrolling and that grouped view handles many sections cleanly.',
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
          'A spinner and "Loading checklist…" message is displayed.',
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
      { text: 'Whole milk', checked: false, tag: 'Dairy' },
      { text: 'Eggs (×12)', checked: false, tag: 'Dairy' },
      { text: 'Apples', checked: false, tag: 'Produce' },
      { text: 'Sourdough bread', checked: false, tag: 'Bakery' },
      { text: 'Butter', checked: false },
    ];

    return html`
      <div style="max-width: 700px; padding: 20px;">
        <h3 style="margin-top: 0;">Interactive Checklist Testing</h3>
        <p>
          <strong>Open the browser developer tools console to see the action logs.</strong>
        </p>

        <h4>Test scenarios</h4>
        <ul style="margin-bottom: 20px; padding-left: 20px; font-size: 0.9em; color: #555;">
          <li>Check/uncheck items — triggers an API save (no backend in Storybook, so a network error is expected)</li>
          <li>Click the <em>Group</em> button to switch to grouped view</li>
          <li>Click a tag badge to edit it inline; press Enter or click away to save</li>
          <li>Click the <em>+tag</em> badge on "Butter" to add a tag</li>
          <li>Type in the "Add new item…" field and press Enter or click Add</li>
          <li>Click ✕ to remove an item</li>
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
          Interactions that trigger save/load will result in a network error in Storybook — this is expected.
          The UI interactions (toggling view, editing tags, etc.) work without a backend.
        </div>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Fully interactive story for manual QA. Pre-loaded with grocery items. ' +
          'Test checking items, toggling grouped/flat view, editing tags, adding and removing items. ' +
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
