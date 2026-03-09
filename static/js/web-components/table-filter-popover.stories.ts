import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './table-filter-popover.js';
import type { TableColumnDefinition } from './table-data-extractor.js';
import type { SortDirection, ColumnFilterState } from './table-sorter-filterer.js';

const meta: Meta = {
  title: 'Components/TableFilterPopover',
  tags: ['autodocs'],
  component: 'table-filter-popover',
  parameters: {
    docs: {
      description: {
        component: `
Filter popover for wiki-table columns. Automatically selects the appropriate filter type:

- **Checkbox filter**: For text columns with 15 or fewer unique values
- **Text search filter**: For text columns with more than 15 unique values
- **Range slider filter**: For numeric, currency, and percentage columns

Includes sort controls (ascending/descending/clear) and dismisses on click-outside or ESC.
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const textColumn: TableColumnDefinition = {
  headerText: 'Department',
  typeInfo: { detectedType: 'text', confidenceRatio: 1 },
  columnIndex: 0,
};

const numberColumn: TableColumnDefinition = {
  headerText: 'Quantity',
  typeInfo: { detectedType: 'number', confidenceRatio: 1 },
  columnIndex: 1,
};

const currencyColumn: TableColumnDefinition = {
  headerText: 'Price',
  typeInfo: { detectedType: 'currency', confidenceRatio: 1 },
  columnIndex: 2,
};

const percentageColumn: TableColumnDefinition = {
  headerText: 'Completion',
  typeInfo: { detectedType: 'percentage', confidenceRatio: 1 },
  columnIndex: 3,
};

export const CheckboxFilter: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${textColumn}
      .uniqueValues=${['Engineering', 'Marketing', 'Sales', 'HR', 'Finance']}
      .currentSortDirection=${'none' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Text column with 5 unique values shows checkbox filter. Uncheck values to exclude them. Use Select All/None for bulk operations. Open browser dev tools to see action logs.',
      },
    },
  },
};

export const CheckboxFilterPrePopulated: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${textColumn}
      .uniqueValues=${['Engineering', 'Marketing', 'Sales', 'HR', 'Finance']}
      .currentFilter=${{ kind: 'checkbox', excludedValues: new Set(['HR', 'Finance']) } as ColumnFilterState}
      .currentSortDirection=${'ascending' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Pre-populated state: HR and Finance are excluded, sort is ascending. Shows how the popover looks when reopening with existing filter state.',
      },
    },
  },
};

export const TextSearchFilter: Story = {
  render: () => {
    const manyValues = [
      'New York', 'Los Angeles', 'Chicago', 'Houston', 'Phoenix',
      'Philadelphia', 'San Antonio', 'San Diego', 'Dallas', 'San Jose',
      'Austin', 'Jacksonville', 'Fort Worth', 'Columbus', 'Charlotte',
      'Indianapolis', 'San Francisco', 'Seattle', 'Denver', 'Washington',
    ];

    return html`
      <table-filter-popover
        .columnDefinition=${{
          headerText: 'City',
          typeInfo: { detectedType: 'text', confidenceRatio: 1 },
          columnIndex: 0,
        } as TableColumnDefinition}
        .uniqueValues=${manyValues}
        .currentSortDirection=${'none' as SortDirection}
        .open=${true}
        @sort-direction-changed=${action('sort-direction-changed')}
        @filter-changed=${action('filter-changed')}
        @popover-closed=${action('popover-closed')}
      ></table-filter-popover>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Text column with more than 15 unique values shows text search input instead of checkboxes. Type to filter by substring match. Open browser dev tools to see action logs.',
      },
    },
  },
};

export const RangeFilterNumber: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${numberColumn}
      .uniqueValues=${['10', '25', '50', '75', '100', '150', '200']}
      .numericRange=${{ min: 10, max: 200 }}
      .currentSortDirection=${'none' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Numeric column shows range sliders and min/max inputs. Adjust sliders or type values to filter by range. Open browser dev tools to see action logs.',
      },
    },
  },
};

export const RangeFilterCurrency: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${currencyColumn}
      .uniqueValues=${['$5.00', '$10.00', '$25.00', '$50.00', '$100.00']}
      .numericRange=${{ min: 5, max: 100 }}
      .currentSortDirection=${'none' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Currency column shows range filter with parsed numeric values. The range uses the underlying numeric values, not the formatted strings.',
      },
    },
  },
};

export const RangeFilterPercentage: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${percentageColumn}
      .uniqueValues=${['0%', '25%', '50%', '75%', '100%']}
      .numericRange=${{ min: 0, max: 100 }}
      .currentSortDirection=${'descending' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Percentage column with descending sort active. Shows range filter for percentage values.',
      },
    },
  },
};

export const RangeFilterPrePopulated: Story = {
  render: () => html`
    <table-filter-popover
      .columnDefinition=${numberColumn}
      .uniqueValues=${['10', '25', '50', '75', '100', '150', '200']}
      .numericRange=${{ min: 10, max: 200 }}
      .currentFilter=${{ kind: 'range', min: 50, max: 150 } as ColumnFilterState}
      .currentSortDirection=${'none' as SortDirection}
      .open=${true}
      @sort-direction-changed=${action('sort-direction-changed')}
      @filter-changed=${action('filter-changed')}
      @popover-closed=${action('popover-closed')}
    ></table-filter-popover>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Range filter pre-populated with min=50, max=150. Shows how the popover appears when reopening with an existing range filter.',
      },
    },
  },
};
