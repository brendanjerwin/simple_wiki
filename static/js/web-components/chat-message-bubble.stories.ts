import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { action } from 'storybook/actions';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './chat-message-bubble.js';
import type { ReactionGroup } from './chat-message-bubble.js';

const meta: Meta = {
  title: 'Chat/MessageBubble',
  component: 'chat-message-bubble',
  parameters: {
    layout: 'padded',
    backgrounds: { default: 'dark' },
    docs: {
      description: {
        component: 'A single chat message bubble supporting user/assistant styling, reactions, threading, and edit state.',
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
      content="Hey Claude, can you help me with this page?"
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
      { emoji: '\uD83D\uDC4D', reactors: ['assistant'], count: 1 },
      { emoji: '\uD83C\uDF89', reactors: ['assistant', 'user1'], count: 2 },
      { emoji: '\u2764\uFE0F', reactors: ['assistant'], count: 1 },
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
