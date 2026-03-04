import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import './file-drop-zone.js';

const meta: Meta = {
  title: 'Components/FileDropZone',
  tags: ['autodocs'],
  component: 'file-drop-zone',
  parameters: {
    docs: {
      description: {
        component: `
A drag-and-drop file upload zone that wraps content via a \`<slot>\`.

**Features:**
- Wraps any content (e.g., a textarea) and overlays drag-and-drop functionality
- Visual overlay on drag enter with "Drop file to upload" indicator
- Uploads files via gRPC \`FileStorageService.uploadFile\`
- Dispatches \`file-uploaded\` custom event with upload URL, filename, and image flag
- File size validation against \`max-upload-mb\`
- Disabled state when \`allow-uploads\` is false

**Usage:**
\`\`\`html
<file-drop-zone allow-uploads max-upload-mb="10">
  <textarea>Your content here</textarea>
</file-drop-zone>
\`\`\`
        `,
      },
    },
  },
  argTypes: {
    allowUploads: {
      control: 'boolean',
      description: 'Whether file uploads are enabled',
    },
    maxUploadMb: {
      control: 'number',
      description: 'Maximum file size in megabytes',
    },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <file-drop-zone>
      <textarea rows="8" style="width: 100%; box-sizing: border-box;">
Uploads are disabled by default. Drag-and-drop is ignored in this state.
      </textarea>
    </file-drop-zone>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Default state with uploads disabled. Dragging files over this zone has no effect.',
      },
    },
  },
};

export const UploadsEnabled: Story = {
  render: () => html`
    <file-drop-zone
      allow-uploads
      max-upload-mb="10"
      @file-uploaded=${action('file-uploaded')}
    >
      <textarea rows="8" style="width: 100%; box-sizing: border-box;">
Uploads are enabled. Drag a file over this area to see the drop overlay.
The file will be uploaded via gRPC when dropped.
      </textarea>
    </file-drop-zone>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Uploads enabled with a 10 MB limit. Drag a file over the textarea to see the overlay. Open browser dev tools to see the file-uploaded action log.',
      },
    },
  },
};

export const DraggingState: Story = {
  render: () => {
    // Manually set the dragging state to preview the overlay
    const setDragging = (e: Event) => {
      const el = e.target as HTMLElement;
      const dropZone = el.closest('file-drop-zone');
      if (dropZone) {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- setting internal state for storybook preview
        (dropZone as unknown as { dragging: boolean }).dragging = true;
      }
    };

    return html`
      <file-drop-zone allow-uploads @firstUpdated=${setDragging} .dragging=${true}>
        <textarea rows="8" style="width: 100%; box-sizing: border-box;">
This shows the dragging overlay state.
        </textarea>
      </file-drop-zone>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Shows the visual overlay that appears when a file is being dragged over the zone. The overlay has a dashed blue border and a cloud upload icon.',
      },
    },
  },
};

export const UploadingState: Story = {
  render: () => html`
    <file-drop-zone allow-uploads .uploading=${true}>
      <textarea rows="8" style="width: 100%; box-sizing: border-box;">
This shows the uploading overlay state that appears while a file is being uploaded.
      </textarea>
    </file-drop-zone>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Shows the semi-transparent overlay with "Uploading..." text that appears during file upload.',
      },
    },
  },
};

export const WithRichContent: Story = {
  render: () => html`
    <file-drop-zone
      allow-uploads
      max-upload-mb="25"
      @file-uploaded=${action('file-uploaded')}
    >
      <div style="padding: 16px; border: 1px solid #ddd; border-radius: 4px; min-height: 200px;">
        <h3>Rich Content Area</h3>
        <p>The drop zone wraps any content via its slot. This could be a rich text editor, a form, or any other content.</p>
        <textarea rows="4" style="width: 100%; box-sizing: border-box;">Edit me while also being able to drop files...</textarea>
      </div>
    </file-drop-zone>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Demonstrates how the drop zone wraps arbitrary content. The slot-based approach preserves existing content while adding drag-and-drop upload capability.',
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <p><strong>Interactive Testing Story</strong></p>
      <p>Drag a file over the textarea to test the upload flow. Open the browser developer tools console to see the action logs.</p>
      <p>Note: The gRPC upload will fail in Storybook (no backend), but you can observe the drag states and error handling.</p>
      <file-drop-zone
        allow-uploads
        max-upload-mb="5"
        @file-uploaded=${action('file-uploaded')}
      >
        <textarea rows="10" style="width: 100%; box-sizing: border-box;">
Try dragging a file here.

- Files under 5 MB will attempt upload (which will fail without a backend)
- Files over 5 MB will show a validation error
- The dragging overlay appears on drag enter
- The uploading overlay appears during upload
        </textarea>
      </file-drop-zone>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Test drag-and-drop interactions. Open browser dev tools to see action logs. Drag files to test the overlay states, file size validation, and error handling.',
      },
    },
  },
};
