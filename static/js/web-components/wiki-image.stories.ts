import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './wiki-image.js';

const meta: Meta = {
  title: 'Components/WikiImage',
  tags: ['autodocs'],
  component: 'wiki-image',
  parameters: {
    docs: {
      description: {
        component: `
Displays images from wiki content with click-to-open-in-new-tab behavior.

**Features:**
- 80% max-width with centered display
- Subtle shadow and border-radius
- Hover effect with enhanced shadow
- Click to open full image in new tab

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
