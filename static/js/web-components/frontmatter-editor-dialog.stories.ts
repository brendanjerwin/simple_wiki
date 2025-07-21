import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import './frontmatter-editor-dialog.js';

const meta: Meta = {
  title: 'Components/FrontmatterEditorDialog',
  tags: ['autodocs'],
  component: 'frontmatter-editor-dialog',
  argTypes: {
    page: {
      control: 'text',
      description: 'The page identifier',
    },
    open: {
      control: 'boolean',
      description: 'Whether the dialog is open',
    },
  },
  parameters: {
    layout: 'fullscreen',
  },
};

export default meta;
type Story = StoryObj;

// Note: These stories show the visual states of the dialog component.
// The actual functionality requires a gRPC backend connection which
// is not available in Storybook isolation.

export const Closed: Story = {
  args: {
    page: 'sample-page',
    open: false,
  },
  render: (args) => html`
    <frontmatter-editor-dialog 
      .page="${args.page}"
      .open="${args.open}">
    </frontmatter-editor-dialog>
  `,
};

export const Open: Story = {
  args: {
    page: 'sample-page',
    open: true,
  },
  render: (args) => {
    // Create a mock component that simulates the dialog states
    return html`
      <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 1000;">
        <div style="position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: white; border-radius: 8px; box-shadow: 0 10px 40px rgba(0,0,0,0.3); width: 90%; max-width: 800px; max-height: 90%; display: flex; flex-direction: column;">
          <div style="display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid #e0e0e0;">
            <h2 style="margin: 0; font-size: 18px; font-weight: 600; color: #333;">Edit Frontmatter - ${args.page}</h2>
            <button style="background: none; border: none; font-size: 20px; cursor: pointer; color: #666;">✕</button>
          </div>
          <div style="flex: 1; padding: 20px; overflow-y: auto; min-height: 300px;">
            <div style="display: flex; align-items: center; justify-content: center; min-height: 200px; color: #666; font-style: italic;">
              Dialog content would load frontmatter data via gRPC.<br>
              In Storybook isolation, the actual editor components are not visible<br>
              because they require a backend connection.
            </div>
          </div>
          <div style="display: flex; gap: 12px; padding: 16px 20px; border-top: 1px solid #e0e0e0; justify-content: flex-end;">
            <button style="background: #6c757d; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">Cancel</button>
            <button style="background: #007bff; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">Save</button>
          </div>
        </div>
      </div>
    `;
  },
};

export const LoadingState: Story = {
  args: {
    page: 'loading-page',
    open: true,
  },
  render: (args) => html`
    <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 1000;">
      <div style="position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: white; border-radius: 8px; box-shadow: 0 10px 40px rgba(0,0,0,0.3); width: 90%; max-width: 800px; max-height: 90%; display: flex; flex-direction: column;">
        <div style="display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid #e0e0e0;">
          <h2 style="margin: 0; font-size: 18px; font-weight: 600; color: #333;">Edit Frontmatter - ${args.page}</h2>
          <button style="background: none; border: none; font-size: 20px; cursor: pointer; color: #666;">✕</button>
        </div>
        <div style="flex: 1; padding: 20px; overflow-y: auto;">
          <div style="display: flex; align-items: center; justify-content: center; min-height: 200px; color: #666; font-size: 16px;">
            Loading frontmatter data...
          </div>
        </div>
        <div style="display: flex; gap: 12px; padding: 16px 20px; border-top: 1px solid #e0e0e0; justify-content: flex-end;">
          <button style="background: #6c757d; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;" disabled>Cancel</button>
          <button style="background: #007bff; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;" disabled>Save</button>
        </div>
      </div>
    </div>
  `,
};

export const ErrorState: Story = {
  args: {
    page: 'error-page',
    open: true,
  },
  render: (args) => html`
    <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.5); z-index: 1000;">
      <div style="position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); background: white; border-radius: 8px; box-shadow: 0 10px 40px rgba(0,0,0,0.3); width: 90%; max-width: 800px; max-height: 90%; display: flex; flex-direction: column;">
        <div style="display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; border-bottom: 1px solid #e0e0e0;">
          <h2 style="margin: 0; font-size: 18px; font-weight: 600; color: #333;">Edit Frontmatter - ${args.page}</h2>
          <button style="background: none; border: none; font-size: 20px; cursor: pointer; color: #666;">✕</button>
        </div>
        <div style="flex: 1; padding: 20px; overflow-y: auto;">
          <div style="display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 8px; min-height: 200px; color: #dc3545; font-size: 16px;">
            <div>Failed to load frontmatter</div>
            <div style="font-size: 14px; color: #666;">Network error: Could not connect to server</div>
          </div>
        </div>
        <div style="display: flex; gap: 12px; padding: 16px 20px; border-top: 1px solid #e0e0e0; justify-content: flex-end;">
          <button style="background: #6c757d; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">Cancel</button>
          <button style="background: #007bff; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;" disabled>Save</button>
        </div>
      </div>
    </div>
  `,
};