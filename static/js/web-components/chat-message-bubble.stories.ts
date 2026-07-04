import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './chat-message-bubble.js';
import type { ReactionGroup } from './chat-message-bubble.js';
import type { ToolCallState, PlanEntryState } from './page-chat-panel.js';

const meta: Meta = {
  title: 'Chat/MessageBubble',
  component: 'chat-message-bubble',
  parameters: {
    layout: 'padded',
    backgrounds: { default: 'dark' },
    docs: {
      description: {
        component: 'A single chat message bubble supporting user/assistant styling, reactions, threading, edit state, live tool-use progress, and plan display.',
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const assistantHtml = '<p>Of course! I can see this page has a <strong>checklist</strong> and some <em>frontmatter</em>. What would you like to change?</p>';
const editedHtml = "<p>I've updated the description. Let me know if you'd like further changes.</p>";
const reactionsHtml = '<p>Done! The page has been updated with the new inventory items.</p>';
const replyHtml = '<p>Regarding your earlier question about the frontmatter...</p>';
const longHtml = "<p>Here's a comprehensive analysis of your page structure:</p><ul><li>The frontmatter contains 12 fields including inventory tracking</li><li>There are 3 checklists with a total of 47 items</li><li>The blog section has 5 published articles</li><li>Several template macros are in use: LinkTo, Checklist, and Blog</li></ul><p>I'd recommend organizing the checklists by priority and archiving completed items to keep the page manageable.</p>";

export const UserMessage: Story = {
  render: () => html`
    <chat-message-bubble
      message-id="msg-1"
      .sender=${Sender.USER}
      sender-name="Brendan"
      content="Hey Dorium, can you help me with this page?"
    ></chat-message-bubble>
  `,
};

export const AssistantMessage: Story = {
  render: () => html`
    <chat-message-bubble
      message-id="msg-2"
      .sender=${Sender.ASSISTANT}
      .renderedHtml=${assistantHtml}
    ></chat-message-bubble>
  `,
};

export const EditedMessage: Story = {
  render: () => html`
    <chat-message-bubble
      message-id="msg-3"
      .sender=${Sender.ASSISTANT}
      .renderedHtml=${editedHtml}
      ?edited=${true}
    ></chat-message-bubble>
  `,
};

export const WithReactions: Story = {
  render: () => {
    const reactions: ReactionGroup[] = [
      { emoji: '👍', reactors: ['assistant'], count: 1 },
      { emoji: '🎉', reactors: ['assistant', 'user1'], count: 2 },
      { emoji: '❤️', reactors: ['assistant'], count: 1 },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-4"
        .sender=${Sender.ASSISTANT}
        .renderedHtml=${reactionsHtml}
        .reactions=${reactions}
      ></chat-message-bubble>
    `;
  },
};

export const WithReplyLink: Story = {
  render: () => html`
    <chat-message-bubble
      message-id="msg-6"
      .sender=${Sender.ASSISTANT}
      .renderedHtml=${replyHtml}
      reply-to-id="msg-1"
      @scroll-to-message=${action('scroll-to-message')}
    ></chat-message-bubble>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Message with a reply-to link. Click the reply indicator to trigger a scroll-to-message event. Open browser dev tools to see the action log.',
      },
    },
  },
};

export const LongContent: Story = {
  render: () => html`
    <div style="max-width: 350px;">
      <chat-message-bubble
        message-id="msg-7"
        .sender=${Sender.ASSISTANT}
        .renderedHtml=${longHtml}
      ></chat-message-bubble>
    </div>
  `,
};

export const LiveToolCall: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-live-1',
        title: 'Reading page content',
        status: 'in_progress',
        kind: 'read',
        detail: '/wiki/InventoryPage',
        startedAtMs: Date.now() - 4200,
      },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-live-tc"
        .sender=${Sender.ASSISTANT}
        content=""
        .toolCalls=${toolCalls}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A tool call in the in_progress state renders as a live expanded row showing the title, detail path, and elapsed time. The elapsed timer ticks every second.',
      },
    },
  },
};

export const PendingToolCall: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-pending-1',
        title: 'Searching wiki',
        status: 'pending',
        kind: 'search',
        detail: '',
        startedAtMs: Date.now(),
      },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-pending-tc"
        .sender=${Sender.ASSISTANT}
        content=""
        .toolCalls=${toolCalls}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A pending tool call also renders in the live expanded view (not yet started but queued).',
      },
    },
  },
};

export const CompletedToolCall: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-done-1',
        title: 'Read InventoryPage',
        status: 'completed',
        kind: 'read',
        detail: '/wiki/InventoryPage',
        startedAtMs: Date.now() - 3000,
      },
      {
        toolCallId: 'tc-done-2',
        title: 'Search for matching items',
        status: 'completed',
        kind: 'search',
        detail: 'query: hammer',
        startedAtMs: Date.now() - 1500,
      },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-done-tc"
        .sender=${Sender.ASSISTANT}
        .renderedHtml=${'<p>Done! Found 3 matching items in the inventory.</p>'}
        .toolCalls=${toolCalls}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Completed tool calls collapse to compact single-line pills showing only the status icon and title.',
      },
    },
  },
};

export const FailedToolCall: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-fail-1',
        title: 'Execute shell command',
        status: 'failed',
        kind: 'execute',
        detail: 'Exit code 127: command not found',
        startedAtMs: Date.now() - 800,
      },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-fail-tc"
        .sender=${Sender.ASSISTANT}
        .renderedHtml=${'<p>I encountered an error while trying to run the command.</p>'}
        .toolCalls=${toolCalls}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'A failed tool call renders as a compact pill with the ❌ icon.',
      },
    },
  },
};

export const MixedToolCalls: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-mix-1',
        title: 'Read InventoryPage',
        status: 'completed',
        kind: 'read',
        detail: '/wiki/InventoryPage',
        startedAtMs: Date.now() - 8000,
      },
      {
        toolCallId: 'tc-mix-2',
        title: 'Search for hammer',
        status: 'completed',
        kind: 'search',
        detail: 'query: hammer',
        startedAtMs: Date.now() - 6000,
      },
      {
        toolCallId: 'tc-mix-3',
        title: 'Updating frontmatter',
        status: 'in_progress',
        kind: 'edit',
        detail: 'wiki/InventoryPage — setting quantity: 5',
        startedAtMs: Date.now() - 1200,
      },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-mixed-tc"
        .sender=${Sender.ASSISTANT}
        content=""
        .toolCalls=${toolCalls}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Mixed tool calls: completed ones collapse to pills, in-progress ones expand with detail and elapsed time.',
      },
    },
  },
};

export const WithPlan: Story = {
  render: () => {
    const plan: PlanEntryState[] = [
      { content: 'Read the inventory page to understand the current structure', status: 'completed', priority: 'high' },
      { content: 'Search for the item matching "hammer"', status: 'completed', priority: 'high' },
      { content: 'Update the quantity field in frontmatter', status: 'in_progress', priority: 'high' },
      { content: 'Confirm the update was saved correctly', status: 'pending', priority: 'medium' },
      { content: 'Reply to the user with a summary', status: 'pending', priority: 'low' },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-plan"
        .sender=${Sender.ASSISTANT}
        content=""
        .plan=${plan}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'The plan block lists agent tasks with per-entry status glyphs: ☑ completed, 🔄 in_progress, ☐ pending.',
      },
    },
  },
};

export const LiveToolCallWithPlan: Story = {
  render: () => {
    const toolCalls: ToolCallState[] = [
      {
        toolCallId: 'tc-combo-1',
        title: 'Read InventoryPage',
        status: 'completed',
        kind: 'read',
        detail: '/wiki/InventoryPage',
        startedAtMs: Date.now() - 6000,
      },
      {
        toolCallId: 'tc-combo-2',
        title: 'Updating frontmatter',
        status: 'in_progress',
        kind: 'edit',
        detail: 'wiki/InventoryPage — quantity: 5',
        startedAtMs: Date.now() - 900,
      },
    ];
    const plan: PlanEntryState[] = [
      { content: 'Read the inventory page', status: 'completed', priority: 'high' },
      { content: 'Update quantity field', status: 'in_progress', priority: 'high' },
      { content: 'Confirm and reply', status: 'pending', priority: 'medium' },
    ];
    return html`
      <chat-message-bubble
        message-id="msg-combo"
        .sender=${Sender.ASSISTANT}
        content=""
        .toolCalls=${toolCalls}
        .plan=${plan}
      ></chat-message-bubble>
    `;
  },
  parameters: {
    docs: {
      description: {
        story: 'Live tool call progress and a plan block together — the typical view during an active agent turn.',
      },
    },
  },
};
