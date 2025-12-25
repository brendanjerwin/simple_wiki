import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from '@storybook/addon-actions';
import './wiki-image.js';

const meta: Meta = {
  title: 'Components/WikiImage',
  tags: ['autodocs'],
  component: 'wiki-image',
  parameters: {
    docs: {
      description: {
        component: `
Displays images from wiki content with a modern tools overlay panel.

**Features:**
- 80% max-width with centered display
- Subtle shadow and border-radius
- Hover effect with enhanced shadow
- Tools panel fades in on hover (desktop) or tap (mobile)
- Full-width gradient overlay with outline SVG icons
- Tools: Open in new tab, Download, Copy image to clipboard

**Usage:**
Images are automatically rendered as \`<wiki-image>\` elements from markdown:
\`\`\`markdown
![Alt text](image.jpg)
![Alt text](image.jpg "Optional title")
\`\`\`
        `,
      },
    },
  },
  argTypes: {
    src: { control: 'text' },
    alt: { control: 'text' },
    title: { control: 'text' },
    toolsOpen: { control: 'boolean' },
  },
};

export default meta;
type Story = StoryObj;

export const Default: Story = {
  render: () => html`
    <wiki-image
      src="https://picsum.photos/800/400"
      alt="Sample image from picsum.photos"
    ></wiki-image>
  `,
};

export const WithTitle: Story = {
  render: () => html`
    <wiki-image
      src="https://picsum.photos/800/400"
      alt="Sample image with tooltip"
      title="Click to view full size"
    ></wiki-image>
  `,
};

export const SmallImage: Story = {
  render: () => html`
    <wiki-image
      src="https://picsum.photos/200/200"
      alt="Small square image"
    ></wiki-image>
  `,
};

export const MultipleImages: Story = {
  render: () => html`
    <div>
      <p>Here is some text before the image.</p>
      <wiki-image
        src="https://picsum.photos/600/300"
        alt="First image"
      ></wiki-image>
      <p>Text between images showing natural spacing.</p>
      <wiki-image
        src="https://picsum.photos/600/300"
        alt="Second image"
      ></wiki-image>
      <p>And some text after.</p>
    </div>
  `,
};

export const WithToolsOpen: Story = {
  render: () => html`
    <wiki-image
      src="https://picsum.photos/800/400"
      alt="Image with tools panel visible"
      tools-open
    ></wiki-image>
  `,
  parameters: {
    docs: {
      description: {
        story: `
Shows the tools panel in its visible state. The panel features a subtle gradient
overlay spanning the full width of the image, with outline SVG icons for a clean,
modern appearance.

**Available tools:**
- **Open in new tab**: Opens the full image in a new browser tab
- **Download**: Downloads the image file directly
- **Copy image**: Copies the image to clipboard for pasting into other apps
        `,
      },
    },
  },
};

export const HoverToReveal: Story = {
  render: () => html`
    <div style="padding: 20px;">
      <p><strong>Hover over the image to reveal the tools panel.</strong></p>
      <p>On mobile devices, tap the image to open the tools. Use the X button or tap outside to close.</p>
      <wiki-image
        src="https://picsum.photos/800/400"
        alt="Hover to see tools"
      ></wiki-image>
    </div>
  `,
  parameters: {
    docs: {
      description: {
        story: `
Demonstrates the hover-to-reveal behavior on desktop. The tools panel fades in
when you hover over the image.

**Mobile behavior:**
- Tap the image to open the tools panel
- A close bar with an X button appears at the top (only on touch devices)
- Tap the X button or anywhere outside the image to close
        `,
      },
    },
  },
};

export const InteractiveTesting: Story = {
  render: () => {
    // Log actions for testing
    const logOpen = action('open-new-tab');
    const logDownload = action('download');
    const logCopy = action('copy-image');

    return html`
      <div style="padding: 20px;">
        <p><strong>Interactive Testing Story</strong></p>
        <p>Open the browser developer tools console to see action logs when clicking tools.</p>
        <wiki-image
          src="https://picsum.photos/800/400"
          alt="Interactive test image"
          tools-open
          @click=${(e: Event) => {
            // Use composedPath to get the actual target from shadow DOM
            const path = e.composedPath();
            const target = path[0] as HTMLElement;
            if (target instanceof HTMLButtonElement) {
              const ariaLabel = target.getAttribute('aria-label');
              if (ariaLabel === 'Open in new tab') {
                logOpen({ src: 'https://picsum.photos/800/400' });
              } else if (ariaLabel === 'Download') {
                logDownload({ filename: '400' });
              } else if (ariaLabel === 'Copy image') {
                logCopy({ src: 'https://picsum.photos/800/400' });
              }
            }
          }}
        ></wiki-image>
      </div>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: `
Test the tools panel interactively. Click each tool button and check the Actions
panel in Storybook to see the logged events. This story is useful for verifying
that all tool buttons are properly wired up.

**Note:** The actual tool actions (opening new tab, downloading, copying) will
still execute in addition to logging the action.
        `,
      },
    },
  },
};
