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
// In Storybook, we can manually set the component's internal state properties
// to demonstrate different UI states without requiring a gRPC backend.

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

export const LoadingState: Story = {
  args: {
    page: 'loading-page',
    open: true,
  },
  render: (args) => {
    // Create element and manually set its internal loading state
    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${true}">
    </frontmatter-editor-dialog>
    `;
  },
};

export const ErrorState: Story = {
  args: {
    page: 'error-page',
    open: true,
  },
  render: (args) => html`
    <frontmatter-editor-dialog 
      .page="${args.page}"
      .open="${args.open}"
      .loading="${false}"
      .error="${'Network error: Could not connect to server'}">
    </frontmatter-editor-dialog>
  `,
};

export const WithFrontmatterData: Story = {
  args: {
    page: 'content-page',
    open: true,
  },
  render: (args) => {
    // Simulate component with loaded frontmatter data
    const mockFrontmatterData = {
      title: 'Sample Page Title',
      author: 'John Doe',
      date: '2024-01-15',
      tags: ['documentation', 'example', 'storybook'],
      category: 'tutorial',
      published: true,
      metadata: {
        version: '1.0',
        lastModified: '2024-01-15T10:30:00Z'
      }
    };

    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${false}"
        .workingFrontmatter="${mockFrontmatterData}">
      </frontmatter-editor-dialog>
    `;
  },
};

export const SavingState: Story = {
  args: {
    page: 'saving-page',
    open: true,
  },
  render: (args) => {
    const mockFrontmatterData = {
      title: 'Page Being Saved',
      author: 'Jane Smith',
      status: 'draft'
    };

    return html`
      <frontmatter-editor-dialog 
        .page="${args.page}"
        .open="${args.open}"
        .loading="${false}"
        .saving="${true}"
        .workingFrontmatter="${mockFrontmatterData}">
      </frontmatter-editor-dialog>
    `;
  },
};