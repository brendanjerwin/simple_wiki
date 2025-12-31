/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './page-import-dialog.js';
import { AugmentErrorService } from './augment-error-service.js';
import { ArrayOpType, type PageImportRecord } from '../gen/api/v1/page_import_pb.js';

const meta: Meta = {
  title: 'Components/PageImportDialog',
  tags: ['autodocs'],
  component: 'page-import-dialog',
  argTypes: {
    open: {
      control: 'boolean',
      description: 'Whether the dialog is open',
    },
  },
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
Modal dialog for importing pages from CSV files.

**Workflow Steps:**
1. **Upload**: File upload via drag-drop or file picker
2. **Validating**: Loading state while parsing CSV
3. **Preview**: Review records with navigation and error filtering
4. **Importing**: Loading state while import job runs
5. **Complete**: Summary with link to report page

**Features:**
- Drag and drop file upload
- CSV validation and preview
- Record-by-record navigation
- Error filtering
- NEW/UPDATE badges for page status
- DELETE/REMOVE operations for fields
- Import progress feedback
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

// Mock data for stories
const createMockRecord = (overrides: Partial<PageImportRecord> = {}): PageImportRecord => ({
  $typeName: 'api.v1.PageImportRecord',
  rowNumber: 1,
  identifier: 'sample_page',
  template: 'default',
  frontmatter: {},
  arrayOps: [],
  fieldsToDelete: [],
  pageExists: false,
  validationErrors: [],
  warnings: [],
  ...overrides,
} as PageImportRecord);

const mockRecordsNoErrors: PageImportRecord[] = [
  createMockRecord({
    rowNumber: 1,
    identifier: 'screwdriver_phillips',
    template: 'inv_item',
    frontmatter: {
      title: 'Phillips Screwdriver',
      'inventory.container': 'tool_drawer',
      description: 'Standard #2 Phillips head screwdriver',
    },
    pageExists: false,
  }),
  createMockRecord({
    rowNumber: 2,
    identifier: 'hammer_claw',
    template: 'inv_item',
    frontmatter: {
      title: 'Claw Hammer',
      'inventory.container': 'tool_drawer',
      weight: '16oz',
    },
    pageExists: true,
  }),
  createMockRecord({
    rowNumber: 3,
    identifier: 'drill_cordless',
    template: 'inv_item',
    frontmatter: {
      title: 'Cordless Drill',
      'inventory.container': 'garage_shelf',
      brand: 'DeWalt',
    },
    pageExists: false,
  }),
];

const mockRecordsWithErrors: PageImportRecord[] = [
  createMockRecord({
    rowNumber: 1,
    identifier: 'valid_item',
    template: 'inv_item',
    frontmatter: { title: 'Valid Item' },
    pageExists: false,
  }),
  createMockRecord({
    rowNumber: 2,
    identifier: 'bad!!identifier',
    template: '',
    frontmatter: {},
    pageExists: false,
    validationErrors: ['identifier contains invalid characters', 'template is required'],
  }),
  createMockRecord({
    rowNumber: 3,
    identifier: '',
    template: 'inv_item',
    frontmatter: { title: 'Missing ID' },
    pageExists: false,
    validationErrors: ['identifier is required'],
  }),
  createMockRecord({
    rowNumber: 4,
    identifier: 'another_valid',
    template: 'inv_item',
    frontmatter: { title: 'Another Valid' },
    pageExists: true,
  }),
];

const mockRecordsAllErrors: PageImportRecord[] = [
  createMockRecord({
    rowNumber: 1,
    identifier: 'bad!!chars',
    template: '',
    frontmatter: {},
    pageExists: false,
    validationErrors: ['identifier contains invalid characters', 'template is required'],
  }),
  createMockRecord({
    rowNumber: 2,
    identifier: '',
    template: 'inv_item',
    frontmatter: {},
    pageExists: false,
    validationErrors: ['identifier is required'],
  }),
  createMockRecord({
    rowNumber: 3,
    identifier: 'duplicate_page',
    template: 'inv_item',
    frontmatter: {},
    pageExists: false,
    validationErrors: ['page already imported in this batch'],
  }),
];

const mockRecordsWithDeleteOps: PageImportRecord[] = [
  createMockRecord({
    rowNumber: 1,
    identifier: 'item_with_deletes',
    template: 'inv_item',
    frontmatter: {
      title: 'Updated Item',
      'inventory.container': 'new_location',
    },
    fieldsToDelete: ['old_field', 'deprecated_property'],
    arrayOps: [
      { $typeName: 'api.v1.ArrayOperation', fieldPath: 'tags', operation: ArrayOpType.ENSURE_EXISTS, value: 'updated' } as any,
      { $typeName: 'api.v1.ArrayOperation', fieldPath: 'tags', operation: ArrayOpType.DELETE_VALUE, value: 'old-tag' } as any,
      { $typeName: 'api.v1.ArrayOperation', fieldPath: 'inventory.items', operation: ArrayOpType.DELETE_VALUE, value: 'removed_item' } as any,
    ],
    pageExists: true,
    warnings: ['Field "old_field" will be permanently deleted'],
  }),
  createMockRecord({
    rowNumber: 2,
    identifier: 'item_with_array_ops',
    template: 'inv_item',
    frontmatter: { title: 'Array Operations Demo' },
    arrayOps: [
      { $typeName: 'api.v1.ArrayOperation', fieldPath: 'categories', operation: ArrayOpType.ENSURE_EXISTS, value: 'electronics' } as any,
      { $typeName: 'api.v1.ArrayOperation', fieldPath: 'categories', operation: ArrayOpType.ENSURE_EXISTS, value: 'tools' } as any,
    ],
    pageExists: false,
  }),
];

const mockRecordsWithWarnings: PageImportRecord[] = [
  createMockRecord({
    rowNumber: 1,
    identifier: 'item_with_warnings',
    template: 'inv_item',
    frontmatter: {
      title: 'Item With Warnings',
      count: '5',
    },
    pageExists: false,
    warnings: [
      'Type coercion: "count" converted from string to number',
      'Field "legacy_id" is deprecated and will be ignored',
    ],
  }),
];

export const Default: Story = {
  args: {
    open: false,
  },
  render: (args) => html`
    <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
      <h3>Page Import Dialog - Closed</h3>
      <p>The dialog in its default closed state.</p>
      <button @click=${() => {
        const dialog = document.querySelector('page-import-dialog') as any;
        dialog?.openDialog();
      }}>Open Dialog</button>
      <page-import-dialog
        ?open=${args['open']}
      ></page-import-dialog>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Shows the dialog in its default closed state. Click the button to open it.',
      },
    },
  },
};

export const UploadState: Story = {
  render: () => {
    const openDialog = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      dialog?.openDialog();
    };

    setTimeout(openDialog, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Upload State</h3>
        <p>Initial state with file picker and drag-drop zone.</p>
        <button @click=${openDialog}>Open Dialog</button>
        <page-import-dialog
        ></page-import-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Dialog open in initial upload state. Users can drag-drop a CSV file or click to browse.',
      },
    },
  },
};

export const UploadWithFileSelected: Story = {
  render: () => {
    const openWithFile = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.openDialog();
        // Simulate a file being selected
        setTimeout(() => {
          dialog.file = new File(['id,template,title\ntest,default,Test'], 'import-data.csv', { type: 'text/csv' });
          dialog.requestUpdate();
        }, 100);
      }
    };

    setTimeout(openWithFile, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Upload With File Selected</h3>
        <p>File selected and ready to parse. Parse button is enabled.</p>
        <button @click=${openWithFile}>Open Dialog</button>
        <page-import-dialog
        ></page-import-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Upload state with a CSV file already selected. The Parse button becomes enabled.',
      },
    },
  },
};

export const ValidatingState: Story = {
  render: () => {
    const openWithValidating = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'validating';
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithValidating, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Validating State</h3>
        <p>Loading spinner during CSV parsing.</p>
        <button @click=${openWithValidating}>Show Validating State</button>
        <page-import-dialog
        ></page-import-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading spinner while the CSV file is being parsed and validated.',
      },
    },
  },
};

export const PreviewNoErrors: Story = {
  render: () => {
    const openWithPreview = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsNoErrors;
        dialog.stats = { total: 3, errors: 0, updates: 1, creates: 2 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = false;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithPreview, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview State - No Errors</h3>
        <p>Records preview with navigation. All records are valid.</p>
        <button @click=${openWithPreview}>Show Preview</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Use the navigation buttons to browse through records. Note the NEW and UPDATE badges.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Preview with records that have no validation errors. Shows record navigation and NEW/UPDATE badges.',
      },
    },
  },
};

export const PreviewWithErrorsFiltered: Story = {
  render: () => {
    const openWithErrors = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsWithErrors;
        dialog.stats = { total: 4, errors: 2, updates: 1, creates: 1 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = true;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithErrors, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview With Errors - Filtered View</h3>
        <p>Shows only error records when "Show errors only" is checked.</p>
        <button @click=${openWithErrors}>Show Errors Only</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Navigation shows "Error X of Y" counter. Validation errors displayed in red.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Preview with some error records, errors-only filter checked. Shows "Error X of Y" counter.',
      },
    },
  },
};

export const PreviewWithErrorsAllRecords: Story = {
  render: () => {
    const openWithAllRecords = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsWithErrors;
        dialog.stats = { total: 4, errors: 2, updates: 1, creates: 1 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = false;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithAllRecords, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview With Errors - All Records</h3>
        <p>All records shown, including both valid and invalid ones.</p>
        <button @click=${openWithAllRecords}>Show All Records</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Uncheck "Show errors only" to see all records. Import button shows valid record count.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Same as above but with filter unchecked to show all records including valid ones.',
      },
    },
  },
};

export const PreviewAllErrors: Story = {
  render: () => {
    const openWithAllErrors = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsAllErrors;
        dialog.stats = { total: 3, errors: 3, updates: 0, creates: 0 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = true;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithAllErrors, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview - All Records Have Errors</h3>
        <p>Every record has validation errors. Import is disabled.</p>
        <button @click=${openWithAllErrors}>Show All Errors</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Import button should be disabled because there are no valid records to import.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'All records have validation errors. The Import button is disabled since there are no valid records.',
      },
    },
  },
};

export const PreviewWithDeleteOperations: Story = {
  render: () => {
    const openWithDeletes = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsWithDeleteOps;
        dialog.stats = { total: 2, errors: 0, updates: 1, creates: 1 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = false;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithDeletes, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview With Delete Operations</h3>
        <p>Records showing DELETE badges for scalar fields and ENSURE/REMOVE for array items.</p>
        <button @click=${openWithDeletes}>Show Delete Operations</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Navigate between records to see field deletions and array operations (ENSURE/REMOVE).
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Records showing DELETE badges for scalar fields to be removed and ENSURE/REMOVE for array item operations.',
      },
    },
  },
};

export const PreviewWithWarnings: Story = {
  render: () => {
    const openWithWarnings = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsWithWarnings;
        dialog.stats = { total: 1, errors: 0, updates: 0, creates: 1 };
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = false;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithWarnings, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview With Warnings</h3>
        <p>Records with non-blocking warnings (type coercions, deprecations).</p>
        <button @click=${openWithWarnings}>Show Warnings</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Warnings are informational and don't prevent import. Shown in yellow.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Records with non-blocking warnings like type coercions. Warnings are displayed in yellow.',
      },
    },
  },
};

export const PreviewWithParsingErrors: Story = {
  render: () => {
    const openWithParsingErrors = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'preview';
        dialog.records = mockRecordsNoErrors.slice(0, 2);
        dialog.stats = { total: 2, errors: 0, updates: 1, creates: 1 };
        dialog.parsingErrors = [
          'Row 5: Missing required column "identifier"',
          'Row 8: Invalid CSV format - unmatched quote',
        ];
        dialog.currentRecordIndex = 0;
        dialog.showErrorsOnly = false;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithParsingErrors, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Preview With Parsing Errors</h3>
        <p>CSV parsing encountered some rows that couldn't be processed.</p>
        <button @click=${openWithParsingErrors}>Show Parsing Errors</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Parsing errors are shown at the top of the preview. Valid records can still be imported.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows parsing errors at the top when some CSV rows could not be processed.',
      },
    },
  },
};

export const ImportingState: Story = {
  render: () => {
    const openWithImporting = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'importing';
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithImporting, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Importing State</h3>
        <p>Loading spinner during import job execution.</p>
        <button @click=${openWithImporting}>Show Importing State</button>
        <page-import-dialog
        ></page-import-dialog>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the loading spinner while the import job is being executed on the server.',
      },
    },
  },
};

export const CompleteState: Story = {
  render: () => {
    const openWithComplete = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.open = true;
        dialog.dialogState = 'complete';
        dialog.importedCount = 15;
        dialog.requestUpdate();
      }
    };

    setTimeout(openWithComplete, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Complete State</h3>
        <p>Success summary with imported count and link to report page.</p>
        <button @click=${openWithComplete}>Show Complete State</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Shows success message with count and provides link to view the import report.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the success summary after import completes with the count and a link to the report page.',
      },
    },
  },
};

export const ErrorState: Story = {
  render: () => {
    const openWithError = () => {
      const dialog = document.querySelector('page-import-dialog') as any;
      if (dialog) {
        dialog.openDialog();
        dialog.file = new File(['invalid,csv'], 'bad-file.csv', { type: 'text/csv' });
        setTimeout(() => {
          dialog.error = AugmentErrorService.augmentError(
            new window.Error('Failed to connect to server: UNAVAILABLE'),
            'parsing CSV'
          );
          dialog.requestUpdate();
        }, 100);
      }
    };

    setTimeout(openWithError, 100);

    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 400px;">
        <h3>Error State</h3>
        <p>Shows error display when parsing or import fails.</p>
        <button @click=${openWithError}>Show Error State</button>
        <page-import-dialog
        ></page-import-dialog>
        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          Error messages are displayed using the error-display component.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the error display when CSV parsing or import fails due to an error.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    return html`
      <div style="padding: 20px; background: #f5f5f5; min-height: 600px;">
        <h3>Interactive Testing Demo</h3>
        <p><strong>Test Instructions:</strong></p>
        <ul style="margin: 10px 0; padding-left: 20px;">
          <li>Click "Open Import Dialog" to start the workflow</li>
          <li>Test drag-drop by dragging a CSV file onto the drop zone</li>
          <li>Test file picker by clicking "Select CSV File"</li>
          <li>Press Escape to close the dialog</li>
          <li>Click the backdrop to dismiss</li>
        </ul>

        <div style="display: flex; gap: 10px; margin: 15px 0; flex-wrap: wrap;">
          <button @click=${() => {
            const dialog = document.querySelector('page-import-dialog') as any;
            dialog?.openDialog();
          }}>Open Import Dialog</button>

          <button @click=${() => {
            const dialog = document.querySelector('page-import-dialog') as any;
            if (dialog) {
              dialog.open = true;
              dialog.dialogState = 'preview';
              dialog.records = mockRecordsNoErrors;
              dialog.stats = { total: 3, errors: 0, updates: 1, creates: 2 };
              dialog.requestUpdate();
            }
          }}>Jump to Preview</button>

          <button @click=${() => {
            const dialog = document.querySelector('page-import-dialog') as any;
            if (dialog) {
              dialog.open = true;
              dialog.dialogState = 'preview';
              dialog.records = mockRecordsWithErrors;
              dialog.stats = { total: 4, errors: 2, updates: 1, creates: 1 };
              dialog.showErrorsOnly = true;
              dialog.requestUpdate();
            }
          }}>Jump to Errors</button>
        </div>

        <page-import-dialog
        ></page-import-dialog>

        <div style="margin-top: 20px; padding: 15px; background: #fff3cd; border-radius: 4px;">
          <h4 style="margin-top: 0;">Expected Behavior:</h4>
          <ul style="margin: 10px 0; padding-left: 20px;">
            <li>Parse button disabled until file is selected</li>
            <li>Import button disabled when all records have errors</li>
            <li>Error filtering checkbox only shown when errors exist</li>
            <li>Navigation between records with Prev/Next buttons</li>
            <li>Escape key closes dialog</li>
            <li>Backdrop click closes dialog</li>
          </ul>
        </div>

        <p style="margin-top: 15px; font-size: 0.9em; color: #666;">
          <strong>Open the browser developer tools console (F12) to see the action logs.</strong>
          Actual CSV parsing and import require backend connection.
        </p>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Comprehensive interactive testing of the page import dialog. Test the full workflow and different states.',
      },
    },
  },
};
