import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { action } from 'storybook/actions';
import { html } from 'lit';
import './wiki-checklist.js';
import type { ChecklistItem } from './wiki-checklist.js';
import { makeChecklistItem } from './checklist-test-fixtures.js';
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
A fully API-driven interactive checklist component backed by ChecklistService.

**Features:**
- Check/uncheck items (server stamps completed_at + completed_by)
- Inline text editing per item with #tag syntax
- Multiple tags per item via \`#tag\` anywhere in text
- Tag filter bar: click a tag pill to filter visible items
- Add new items with optional tags (e.g. "milk #dairy #fridge")
- Remove items
- Drag-and-drop reordering (single ReorderItem RPC per drop)
- Automatic polling every 10 s with updated_at short-circuit
- Optimistic concurrency with FailedPrecondition retry + toast

**Usage:**
\`\`\`html
<wiki-checklist list-name="grocery_list" page="my-page"></wiki-checklist>
\`\`\`

**Storybook note:** In Storybook, the component has no backend, so stories bypass
the API by setting \`items\` (and optionally \`loading\`, \`error\`, \`filterTags\`)
directly on the element after fixture creation. Use the \`makeChecklistItem\`
helper to build proto-typed items.
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
      makeChecklistItem({ text: 'Milk', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Eggs', checked: true, tags: ['dairy'] }),
      makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
      makeChecklistItem({ text: 'Bananas', checked: true, tags: ['produce'] }),
      makeChecklistItem({ text: 'Sourdough bread', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Butter' }),
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
      makeChecklistItem({ text: 'Buy coffee', checked: true }),
      makeChecklistItem({ text: 'Read emails', checked: true }),
      makeChecklistItem({ text: 'Team stand-up', checked: true }),
      makeChecklistItem({ text: 'Code review', checked: true }),
      makeChecklistItem({ text: 'Deploy to staging', checked: true }),
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
      makeChecklistItem({ text: 'Pick up dry cleaning' }),
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
      makeChecklistItem({ text: 'Whole milk (2L)', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Cheddar cheese', checked: true, tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Greek yogurt', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Fuji apples', tags: ['produce', 'fridge'] }),
      makeChecklistItem({ text: 'Baby spinach', tags: ['produce', 'fridge'] }),
      makeChecklistItem({ text: 'Roma tomatoes', checked: true, tags: ['produce'] }),
      makeChecklistItem({ text: 'Sourdough loaf', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Croissants (x4)', tags: ['bakery'] }),
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
      makeChecklistItem({ text: 'Whole milk (2L)', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Cheddar cheese', checked: true, tags: ['dairy'] }),
      makeChecklistItem({ text: 'Fuji apples', tags: ['produce'] }),
      makeChecklistItem({ text: 'Baby spinach', tags: ['produce', 'fridge'] }),
      makeChecklistItem({ text: 'Sourdough loaf', tags: ['bakery'] }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .filterTags=${['dairy']}
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
      makeChecklistItem({ text: 'Write unit tests', tags: ['dev'] }),
      makeChecklistItem({ text: 'Update README', checked: true }),
      makeChecklistItem({ text: 'Fix TypeScript error', tags: ['dev'] }),
      makeChecklistItem({ text: 'Buy coffee beans' }),
      makeChecklistItem({ text: 'Review PR #42', tags: ['dev'] }),
      makeChecklistItem({ text: 'Call dentist', checked: true }),
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
      makeChecklistItem({ text: 'Whole milk (2L)', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Skimmed milk (1L)', checked: true, tags: ['dairy'] }),
      makeChecklistItem({ text: 'Cheddar cheese', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Greek yogurt', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Butter (250g)', checked: true, tags: ['dairy'] }),
      // Produce
      makeChecklistItem({ text: 'Fuji apples (x6)', tags: ['produce'] }),
      makeChecklistItem({ text: 'Baby spinach', tags: ['produce'] }),
      makeChecklistItem({ text: 'Roma tomatoes', checked: true, tags: ['produce'] }),
      makeChecklistItem({ text: 'Yellow onions', tags: ['produce'] }),
      makeChecklistItem({ text: 'Garlic (bulb)', tags: ['produce'] }),
      // Bakery
      makeChecklistItem({ text: 'Sourdough loaf', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Croissants (x4)', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Bagels (x6)', checked: true, tags: ['bakery'] }),
      // Frozen
      makeChecklistItem({ text: 'Frozen peas (400g)', tags: ['frozen'] }),
      makeChecklistItem({ text: 'Ice cream (vanilla)', tags: ['frozen'] }),
      makeChecklistItem({ text: 'Frozen fish fillets', checked: true, tags: ['frozen'] }),
      // Pantry
      makeChecklistItem({ text: 'Pasta (500g)', tags: ['pantry'] }),
      makeChecklistItem({ text: 'Olive oil (500ml)', tags: ['pantry'] }),
      makeChecklistItem({ text: 'Tinned tomatoes (x4)', checked: true, tags: ['pantry'] }),
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

export const CompletedByAttribution: Story = {
  render: () => {
    const now = Date.now();
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Mow the lawn',
        checked: true,
        completedBy: 'alice@example.com',
        completedAtMs: now - 2 * 60 * 60 * 1000, // 2h ago
      }),
      makeChecklistItem({
        text: 'Take out the trash',
        checked: true,
        completedBy: 'bob@example.com',
        completedAtMs: now - 30 * 60 * 1000, // 30m ago
      }),
      makeChecklistItem({ text: 'Water the plants' }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="chores"
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
          'Checked items show a "Done by X · Nh ago" caption sourced from the server-stamped completed_by + completed_at fields.',
      },
    },
  },
};

export const CompletedByAgent: Story = {
  render: () => {
    const now = Date.now();
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Auto-tag inventory items',
        checked: true,
        automated: true,
        completedBy: 'wiki-cli',
        completedAtMs: now - 15 * 60 * 1000, // 15m ago
      }),
      makeChecklistItem({
        text: 'Daily backup',
        checked: true,
        automated: true,
        completedBy: 'agent:backup-bot',
        completedAtMs: now - 4 * 60 * 60 * 1000, // 4h ago
      }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="automated_tasks"
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
          'Items completed by an automated caller (Tailscale-tagged or x-wiki-is-agent). The caption reads "Done by an agent · Nh ago".',
      },
    },
  },
};

export const MixedAttribution: Story = {
  render: () => {
    const now = Date.now();
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Buy milk',
        checked: true,
        completedBy: 'alice@example.com',
        completedAtMs: now - 60 * 60 * 1000,
      }),
      makeChecklistItem({
        text: 'Sync inventory',
        checked: true,
        automated: true,
        completedBy: 'wiki-cli',
        completedAtMs: now - 5 * 60 * 1000,
      }),
      makeChecklistItem({ text: 'Walk the dog' }),
      makeChecklistItem({
        text: 'Send weekly report',
        checked: true,
        completedBy: 'bob@example.com',
        completedAtMs: now - 10 * 60 * 1000,
      }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="weekly_tasks"
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
          'Mix of human and agent completions in the same list — useful for verifying both captions render side-by-side.',
      },
    },
  },
};

export const OverdueItems: Story = {
  render: () => {
    const now = Date.now();
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Renew passport',
        dueMs: now - 7 * 24 * 60 * 60 * 1000, // 1w overdue
      }),
      makeChecklistItem({
        text: 'File taxes',
        dueMs: now - 60 * 60 * 1000, // 1h overdue
      }),
      makeChecklistItem({
        text: 'Doctor appointment',
        dueMs: now + 2 * 60 * 60 * 1000, // in 2h
      }),
      makeChecklistItem({
        text: 'Past task (completed)',
        checked: true,
        dueMs: now - 24 * 60 * 60 * 1000,
        completedBy: 'alice@example.com',
        completedAtMs: now - 12 * 60 * 60 * 1000,
      }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="deadlines"
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
          'Items with due dates. Past-due items render with the overdue style; checked items do not.',
      },
    },
  },
};

export const WithDescriptions: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Buy oat milk',
        tags: ['dairy'],
        description: 'The brand Kirsten likes — Califia Farms, unsweetened.',
      }),
      makeChecklistItem({
        text: 'Pick up new tires',
        description: 'Discount Tire on Main Street. Appointment is at 2pm.',
      }),
      makeChecklistItem({ text: 'Call electrician' }),
    ];

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <wiki-checklist
          list-name="errands"
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
          'Items can carry an optional description (sub-line content) that renders below the main text.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const groceryItems: ChecklistItem[] = [
      makeChecklistItem({ text: 'Whole milk', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Eggs (x12)', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
      makeChecklistItem({ text: 'Sourdough bread', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Butter' }),
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
          <li>Type in the "Add item..." field with #tag syntax and press Enter or click Add</li>
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

export const TouchDragTesting: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      makeChecklistItem({ text: 'Whole milk', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Eggs (x12)', tags: ['dairy'] }),
      makeChecklistItem({ text: 'Apples', tags: ['produce'] }),
      makeChecklistItem({ text: 'Sourdough bread', tags: ['bakery'] }),
      makeChecklistItem({ text: 'Butter' }),
      makeChecklistItem({ text: 'Greek yogurt', tags: ['dairy', 'fridge'] }),
      makeChecklistItem({ text: 'Bananas', tags: ['produce'] }),
    ];

    return html`
      <div style="max-width: 700px; padding: 20px;">
        <h3 style="margin-top: 0;">Touch Drag Reordering (Mobile QA)</h3>
        <p>
          <strong>Use Chrome DevTools touch emulation or a real mobile device.</strong>
        </p>

        <h4>Test steps</h4>
        <ol style="margin-bottom: 20px; padding-left: 20px; font-size: 0.9em; color: #555;">
          <li>Long-press the drag handle (the dots on the left) -- after ~400ms a ghost should appear</li>
          <li>While holding, drag over other items -- a blue insertion line should follow</li>
          <li>Release to drop -- the item should reorder and persist</li>
          <li>Quick tap on a drag handle -- should NOT trigger drag (timer cancelled)</li>
          <li>Touch and scroll on the list body -- normal scrolling should work (no drag)</li>
          <li>Start a long-press, then move finger >10px before 400ms -- should cancel (scrolling)</li>
        </ol>

        <wiki-checklist
          list-name="grocery_list"
          .items=${items}
          .loading=${false}
          .error=${null}
          @click=${action('checklist-click')}
        ></wiki-checklist>

        <div style="margin-top: 24px; padding: 14px; background: #d1ecf1; border-radius: 6px; font-size: 0.88em; color: #0c5460;">
          <strong>Tip:</strong> In Chrome DevTools, toggle "Device toolbar" (Ctrl+Shift+M) and
          select a mobile device preset to enable touch event emulation.
        </div>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story:
          'Story for testing touch-based long-press-to-drag reordering on mobile devices. ' +
          'Use Chrome DevTools touch emulation or a real touch device. ' +
          'Long-press the drag handle to initiate drag mode, then drag to reorder.',
      },
    },
  },
};

export const OCCRetryToast: Story = {
  render: () => {
    const items: ChecklistItem[] = [
      makeChecklistItem({ text: 'Item A', tags: ['x'] }),
      makeChecklistItem({ text: 'Item B', tags: ['x'] }),
      makeChecklistItem({ text: 'Item C', tags: ['y'] }),
    ];

    // Force the toast visible by setting the internal state after the
    // component upgrades. Stories can reach into the @state property
    // because Lit's decorator-defined state is just a class field.
    const toggleToast = (e: Event) => {
      const target = e.currentTarget as HTMLButtonElement | null;
      const root = target?.parentElement;
      const checklist = root?.querySelector('wiki-checklist') as
        | (HTMLElement & { _occRetryToastVisible?: boolean; requestUpdate?: () => void })
        | null;
      if (checklist) {
        checklist._occRetryToastVisible = !checklist._occRetryToastVisible;
        checklist.requestUpdate?.();
      }
    };

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <button @click=${toggleToast} style="margin-bottom: 12px;">
          Toggle "Edited concurrently" toast
        </button>
        <wiki-checklist
          list-name="shared_list"
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
          'The OCC retry toast — surfaced briefly after a FailedPrecondition triggers a refetch+retry. Click the button to toggle visibility.',
      },
    },
  },
};

export const DebugSyncBadge: Story = {
  render: () => {
    const now = Date.now();
    const items: ChecklistItem[] = [
      makeChecklistItem({
        text: 'Item with debug metadata',
        tags: ['dev'],
        sortOrder: 1000n,
        updatedAtMs: now - 60_000,
      }),
      makeChecklistItem({
        text: 'Another item',
        tags: ['dev'],
        sortOrder: 2000n,
        updatedAtMs: now,
      }),
    ];

    // Stories cannot reliably set localStorage before the page renders, so we
    // do it imperatively then nudge the components to re-render.
    const enableDebug = () => {
      try { globalThis.localStorage.setItem('wiki-checklist-debug', '1'); } catch { /* noop */ }
      document.querySelectorAll('wiki-checklist').forEach(c => {
        const lit = c as HTMLElement & { requestUpdate?: () => void };
        lit.requestUpdate?.();
      });
    };
    const disableDebug = () => {
      try { globalThis.localStorage.removeItem('wiki-checklist-debug'); } catch { /* noop */ }
      document.querySelectorAll('wiki-checklist').forEach(c => {
        const lit = c as HTMLElement & { requestUpdate?: () => void };
        lit.requestUpdate?.();
      });
    };

    return html`
      <div style="max-width: 640px; padding: 20px;">
        <div style="margin-bottom: 12px;">
          <button @click=${enableDebug}>Enable debug</button>
          <button @click=${disableDebug}>Disable debug</button>
        </div>
        <wiki-checklist
          list-name="debug_list"
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
          'When the localStorage flag wiki-checklist-debug = "1" is set, the component renders a per-list sync_token + updated_at line and per-item uid + updated_at + sort_order. Use the buttons to toggle the flag.',
      },
    },
  },
};
