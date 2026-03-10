import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './wiki-table.js';

const meta: Meta = {
  title: 'Components/WikiTable',
  tags: ['autodocs'],
  component: 'wiki-table',
  parameters: {
    docs: {
      description: {
        component: `
Enhances standard HTML tables rendered from markdown with interactive features:

- **Column sorting**: Click sort arrows in headers, or use the filter popover
- **Column filtering**: Click header to open popover with checkbox, range, or text-search filters
- **Smart type detection**: Automatically detects number, currency, percentage, date, and text columns
- **Status bar**: Shows row count with segmented view toggle and pill buttons for filter/sort
- **Filter popover**: Centered popover with sort controls and auto-detected filter type
- **Card view**: Responsive card layout for narrow screens (auto or manual toggle)
- **Scroll shadows**: Visual indicators when table content is scrollable horizontally
- **Progressive enhancement**: Falls back to plain table if JS is disabled
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <wiki-table>
      <table>
        <thead>
          <tr><th>Name</th><th>Department</th><th>Role</th></tr>
        </thead>
        <tbody>
          <tr><td>Alice Johnson</td><td>Engineering</td><td>Senior Dev</td></tr>
          <tr><td>Bob Smith</td><td>Marketing</td><td>Manager</td></tr>
          <tr><td>Charlie Brown</td><td>Engineering</td><td>Junior Dev</td></tr>
          <tr><td>Diana Prince</td><td>Sales</td><td>Director</td></tr>
          <tr><td>Eve Wilson</td><td>Marketing</td><td>Designer</td></tr>
        </tbody>
      </table>
    </wiki-table>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Basic table with text columns. Click a column header to open the filter popover with checkbox filter (fewer than 15 unique values). Use sort arrows on the right side of headers for quick sort cycling.',
      },
    },
  },
};

export const NumericData: Story = {
  render: () => html`
    <wiki-table>
      <table>
        <thead>
          <tr><th>Product</th><th>Price</th><th>Quantity</th><th>Revenue</th></tr>
        </thead>
        <tbody>
          <tr><td>Widget A</td><td>$9.99</td><td>150</td><td>$1,498.50</td></tr>
          <tr><td>Widget B</td><td>$24.50</td><td>75</td><td>$1,837.50</td></tr>
          <tr><td>Gadget X</td><td>$3.75</td><td>500</td><td>$1,875.00</td></tr>
          <tr><td>Gadget Y</td><td>$149.99</td><td>12</td><td>$1,799.88</td></tr>
          <tr><td>Doohickey</td><td>$0.99</td><td>1000</td><td>$990.00</td></tr>
        </tbody>
      </table>
    </wiki-table>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Table with currency and numeric columns. Click a numeric column header to see the range slider filter. Hover over headers to see detected type.',
      },
    },
  },
};

export const MixedTypes: Story = {
  render: () => html`
    <wiki-table>
      <table>
        <thead>
          <tr><th>Task</th><th>Due Date</th><th>Budget</th><th>Completion</th><th>Priority</th></tr>
        </thead>
        <tbody>
          <tr><td>Design mockups</td><td>2024-01-15</td><td>$5,000</td><td>100%</td><td>High</td></tr>
          <tr><td>Backend API</td><td>2024-02-20</td><td>$15,000</td><td>75%</td><td>High</td></tr>
          <tr><td>Frontend UI</td><td>2024-03-10</td><td>$12,000</td><td>50%</td><td>Medium</td></tr>
          <tr><td>Testing</td><td>2024-04-01</td><td>$8,000</td><td>25%</td><td>Low</td></tr>
          <tr><td>Documentation</td><td>2024-04-15</td><td>$3,000</td><td>10%</td><td>Low</td></tr>
          <tr><td>Deployment</td><td>2024-05-01</td><td>$2,000</td><td>0%</td><td>Medium</td></tr>
        </tbody>
      </table>
    </wiki-table>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Table with dates, currencies, percentages, and text. Each column type is auto-detected. Hover headers to see types. Click to open type-appropriate filter popovers.',
      },
    },
  },
};

export const WideTable: Story = {
  render: () => html`
    <div style="max-width: 500px;">
      <wiki-table>
        <table>
          <thead>
            <tr>
              <th>ID</th><th>First Name</th><th>Last Name</th><th>Email</th>
              <th>Department</th><th>Title</th><th>Salary</th><th>Start Date</th>
              <th>Location</th><th>Status</th>
            </tr>
          </thead>
          <tbody>
            <tr><td>1</td><td>Alice</td><td>Johnson</td><td>alice@example.com</td><td>Engineering</td><td>Senior Dev</td><td>$120,000</td><td>2020-03-15</td><td>New York</td><td>Active</td></tr>
            <tr><td>2</td><td>Bob</td><td>Smith</td><td>bob@example.com</td><td>Marketing</td><td>Manager</td><td>$95,000</td><td>2019-07-01</td><td>Chicago</td><td>Active</td></tr>
            <tr><td>3</td><td>Charlie</td><td>Brown</td><td>charlie@example.com</td><td>Sales</td><td>Rep</td><td>$65,000</td><td>2022-01-10</td><td>Denver</td><td>On Leave</td></tr>
          </tbody>
        </table>
      </wiki-table>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Constrained to 500px width to demonstrate horizontal scrolling with scroll shadow indicators.',
      },
    },
  },
};

export const CardView: Story = {
  render: () => html`
    <div style="max-width: 400px;">
      <wiki-table>
        <table>
          <thead>
            <tr><th>Name</th><th>Price</th><th>In Stock</th></tr>
          </thead>
          <tbody>
            <tr><td>Widget</td><td>$9.99</td><td>Yes</td></tr>
            <tr><td>Gadget</td><td>$24.50</td><td>No</td></tr>
            <tr><td>Doohickey</td><td>$1.50</td><td>Yes</td></tr>
          </tbody>
        </table>
      </wiki-table>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Narrow container simulating mobile viewport. Use the segmented view toggle in the status bar. In card view, use the filter and sort pills to access column filters and sorting via a column picker.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    const logSort = action('sort-changed');
    const logFilter = action('filter-changed');
    const logViewToggle = action('view-toggled');

    return html`
      <wiki-table
        @click=${(e: Event) => {
          const target = e.composedPath()[0];
          if (target instanceof HTMLElement) {
            if (target.closest('.sort-arrows')) {
              logSort({ action: 'sort-arrow-click' });
            }
            if (target.closest('.header-main')) {
              logSort({ action: 'header-click-opens-popover' });
            }
            if (target.closest('[aria-label="View mode"]')) {
              logViewToggle({});
            }
            if (target.closest('[aria-label="Clear all filters"]')) {
              logFilter({ action: 'clear-all' });
            }
          }
        }}
      >
        <table>
          <thead>
            <tr><th>Product</th><th>Price</th><th>Stock</th><th>Rating</th></tr>
          </thead>
          <tbody>
            <tr><td>Alpha</td><td>$10.00</td><td>50</td><td>85%</td></tr>
            <tr><td>Beta</td><td>$25.00</td><td>30</td><td>92%</td></tr>
            <tr><td>Gamma</td><td>$5.00</td><td>100</td><td>78%</td></tr>
            <tr><td>Delta</td><td>$50.00</td><td>10</td><td>95%</td></tr>
            <tr><td>Epsilon</td><td>$15.00</td><td>75</td><td>88%</td></tr>
          </tbody>
        </table>
      </wiki-table>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Interactive testing story. Open the browser developer tools console to see action logs. Try: clicking headers to open filter popovers, using sort arrows, toggling card view, filtering with checkboxes/range sliders.',
      },
    },
  },
};

export const EmptyTable: Story = {
  render: () => html`
    <wiki-table>
      <table>
        <thead>
          <tr><th>Name</th><th>Value</th></tr>
        </thead>
        <tbody></tbody>
      </table>
    </wiki-table>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Edge case: table with headers but no data rows.',
      },
    },
  },
};

export const SingleRowTable: Story = {
  render: () => html`
    <wiki-table>
      <table>
        <thead>
          <tr><th>Name</th><th>Value</th><th>Status</th></tr>
        </thead>
        <tbody>
          <tr><td>Only Row</td><td>42</td><td>Active</td></tr>
        </tbody>
      </table>
    </wiki-table>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Edge case: table with only one data row.',
      },
    },
  },
};
