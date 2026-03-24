import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './page-chat-panel.js';
import type { PageChatPanel, ChatMessageState } from './page-chat-panel.js';

const meta: Meta = {
  title: 'Chat/PageChatPanel',
  component: 'page-chat-panel',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: 'The main chat panel component with FAB toggle, message list, input area, and streaming support. Open browser dev tools to see action logs.',
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const sampleMessages: ChatMessageState[] = [
  {
    id: 'msg-1',
    sender: Sender.USER,
    content: 'Can you help me update the inventory on this page?',
    renderedHtml: '',
    timestamp: new Date('2026-03-24T10:00:00'),
    senderName: 'Brendan',
    replyToId: '',
    reactions: [],
    edited: false,
    sequence: 1n,
  },
  {
    id: 'msg-2',
    sender: Sender.ASSISTANT,
    content: '',
    renderedHtml: '<p>Of course! I can see this page has an inventory container. What items would you like to add or move?</p>',
    timestamp: new Date('2026-03-24T10:00:05'),
    senderName: '',
    replyToId: '',
    reactions: [{ emoji: '👍', reactors: ['Brendan'], count: 1 }],
    edited: false,
    sequence: 2n,
  },
  {
    id: 'msg-3',
    sender: Sender.USER,
    content: 'Add a "USB-C Hub" to the desk drawer container',
    renderedHtml: '',
    timestamp: new Date('2026-03-24T10:01:00'),
    senderName: 'Brendan',
    replyToId: '',
    reactions: [],
    edited: false,
    sequence: 3n,
  },
  {
    id: 'msg-4',
    sender: Sender.ASSISTANT,
    content: '',
    renderedHtml: '<p>Done! I\'ve created a new inventory item <strong>USB-C Hub</strong> and placed it in the <a href="/desk_drawer">Desk Drawer</a> container.</p>',
    timestamp: new Date('2026-03-24T10:01:10'),
    senderName: '',
    replyToId: 'msg-3',
    reactions: [],
    edited: true,
    sequence: 4n,
  },
];

export const Default: Story = {
  render: () => html`
    <page-chat-panel page="test-page"></page-chat-panel>
    <p style="padding: 20px; color: #ccc;">The FAB button should be visible in the bottom-right corner. Click it to open the panel.</p>
  `,
};

export const PanelOpenWithMessages: Story = {
  render: () => {
    const el = document.createElement('page-chat-panel') as PageChatPanel;
    el.page = 'test-page';
    el.panelOpen = true;
    el.messages = sampleMessages;
    return el;
  },
};

export const EmptyChat: Story = {
  render: () => {
    const el = document.createElement('page-chat-panel') as PageChatPanel;
    el.page = 'test-page';
    el.panelOpen = true;
    el.messages = [];
    return el;
  },
};

export const ThinkingState: Story = {
  render: () => {
    const el = document.createElement('page-chat-panel') as PageChatPanel;
    el.page = 'test-page';
    el.panelOpen = true;
    el.messages = sampleMessages.slice(0, 3);
    el.waitingForAssistant = true;
    return el;
  },
};

export const ReconnectingState: Story = {
  render: () => {
    const el = document.createElement('page-chat-panel') as PageChatPanel;
    el.page = 'test-page';
    el.panelOpen = true;
    el.messages = sampleMessages.slice(0, 2);
    el.streamState = 'reconnecting';
    return el;
  },
};

export const ErrorState: Story = {
  render: () => {
    const el = document.createElement('page-chat-panel') as PageChatPanel;
    el.page = 'test-page';
    el.panelOpen = true;
    el.messages = [];
    el.streamState = 'disconnected';
    el.error = new Error('Claude is not connected');
    return el;
  },
};
