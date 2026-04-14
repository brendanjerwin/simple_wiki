import { expect, fixture, html } from '@open-wc/testing';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './chat-message-bubble.js';
import type { ChatMessageBubble, ReactionGroup, ScrollToMessageEventDetail } from './chat-message-bubble.js';
import type { ToolCallState } from './page-chat-panel.js';

describe('ChatMessageBubble', () => {
  describe('when rendering a user message', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-1"
          .sender=${Sender.USER}
          content="Hello there"
        ></chat-message-bubble>
      `);
    });

    it('should render with user class', () => {
      const bubble = el.shadowRoot!.querySelector('.bubble');
      expect(bubble!.classList.contains('user')).to.be.true;
    });

    it('should display the content as text', () => {
      const content = el.shadowRoot!.querySelector('.content');
      expect(content!.textContent!.trim()).to.equal('Hello there');
    });
  });

  describe('when rendering an assistant message with HTML', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-2"
          .sender=${Sender.ASSISTANT}
          .renderedHtml=${'<p>Hello <strong>world</strong></p>'}
        ></chat-message-bubble>
      `);
    });

    it('should render with assistant class', () => {
      const bubble = el.shadowRoot!.querySelector('.bubble');
      expect(bubble!.classList.contains('assistant')).to.be.true;
    });

    it('should render HTML content', () => {
      const strong = el.shadowRoot!.querySelector('.content strong');
      expect(strong).to.not.be.null;
      expect(strong!.textContent).to.equal('world');
    });
  });

  describe('when the message is edited', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-3"
          .sender=${Sender.ASSISTANT}
          content="Updated text"
          ?edited=${true}
        ></chat-message-bubble>
      `);
    });

    it('should show the edited indicator', () => {
      const edited = el.shadowRoot!.querySelector('.edited-indicator');
      expect(edited).to.not.be.null;
      expect(edited!.textContent).to.contain('(edited)');
    });
  });

  describe('when the message is not edited', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-4"
          .sender=${Sender.USER}
          content="Original text"
        ></chat-message-bubble>
      `);
    });

    it('should not show the edited indicator', () => {
      const edited = el.shadowRoot!.querySelector('.edited-indicator');
      expect(edited).to.be.null;
    });
  });

  describe('when the message has reactions', () => {
    let el: ChatMessageBubble;
    const reactions: ReactionGroup[] = [
      { emoji: '👍', reactors: ['assistant'], count: 1 },
      { emoji: '❤️', reactors: ['assistant', 'user1'], count: 2 },
    ];

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-5"
          .sender=${Sender.ASSISTANT}
          content="Great idea"
          .reactions=${reactions}
        ></chat-message-bubble>
      `);
    });

    it('should render reaction chips', () => {
      const chips = el.shadowRoot!.querySelectorAll('.reaction-chip');
      expect(chips.length).to.equal(2);
    });

    it('should show count badge for reactions with count > 1', () => {
      const counts = el.shadowRoot!.querySelectorAll('.reaction-count');
      expect(counts.length).to.equal(1);
      expect(counts[0]!.textContent).to.equal('2');
    });
  });

  describe('when the message has no reactions', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-6"
          .sender=${Sender.USER}
          content="Test"
        ></chat-message-bubble>
      `);
    });

    it('should not render the reactions container', () => {
      const reactions = el.shadowRoot!.querySelector('.reactions');
      expect(reactions).to.be.null;
    });
  });

  describe('when the message has a reply-to link', () => {
    let el: ChatMessageBubble;
    let eventDetail: ScrollToMessageEventDetail | null;

    beforeEach(async () => {
      eventDetail = null;
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-7"
          .sender=${Sender.ASSISTANT}
          content="This is a reply"
          reply-to-id="msg-1"
        ></chat-message-bubble>
      `);
      el.addEventListener('scroll-to-message', ((e: CustomEvent<ScrollToMessageEventDetail>) => {
        eventDetail = e.detail;
      }) as EventListener);
    });

    it('should render the reply link', () => {
      const replyLink = el.shadowRoot!.querySelector('.reply-link');
      expect(replyLink).to.not.be.null;
    });

    describe('when the reply link is clicked', () => {
      beforeEach(async () => {
        const replyLink = el.shadowRoot!.querySelector('.reply-link')!;
        (replyLink as HTMLElement).click();
      });

      it('should dispatch scroll-to-message event', () => {
        expect(eventDetail).to.not.be.null;
      });

      it('should include the parent message ID in the event', () => {
        expect(eventDetail!.messageId).to.equal('msg-1');
      });
    });
  });

  describe('when the message has no reply-to link', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-8"
          .sender=${Sender.USER}
          content="Not a reply"
        ></chat-message-bubble>
      `);
    });

    it('should not render the reply link', () => {
      const replyLink = el.shadowRoot!.querySelector('.reply-link');
      expect(replyLink).to.be.null;
    });
  });

  describe('when sender-name is provided', () => {
    let el: ChatMessageBubble;

    beforeEach(async () => {
      el = await fixture(html`
        <chat-message-bubble
          message-id="msg-9"
          .sender=${Sender.USER}
          sender-name="Alice"
          content="Named message"
        ></chat-message-bubble>
      `);
    });

    it('should display the sender name', () => {
      const name = el.shadowRoot!.querySelector('.sender-name');
      expect(name).to.not.be.null;
      expect(name!.textContent).to.equal('Alice');
    });
  });

  describe('tool call rendering', () => {

    describe('when toolCalls is empty', () => {
      let el: ChatMessageBubble;

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-empty"
            .sender=${Sender.ASSISTANT}
            content="No tools"
            .toolCalls=${[]}
          ></chat-message-bubble>
        `);
      });

      it('should not render the tool-calls container', () => {
        const toolCalls = el.shadowRoot!.querySelector('.tool-calls');
        expect(toolCalls).to.be.null;
      });
    });

    describe('when toolCalls has entries', () => {
      let el: ChatMessageBubble;
      const toolCalls: ToolCallState[] = [
        { toolCallId: 'tc-1', title: 'Read File', status: 'complete' },
        { toolCallId: 'tc-2', title: 'Execute Shell', status: 'running' },
        { toolCallId: 'tc-3', title: 'Failed Op', status: 'error' },
      ];

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-multi"
            .sender=${Sender.ASSISTANT}
            content="Used tools"
            .toolCalls=${toolCalls}
          ></chat-message-bubble>
        `);
      });

      it('should render the tool-calls container', () => {
        const container = el.shadowRoot!.querySelector('.tool-calls');
        expect(container).to.not.be.null;
      });

      it('should render one pill per tool call', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        expect(pills.length).to.equal(3);
      });

      it('should display the tool call title', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        expect(pills[0]!.textContent).to.contain('Read File');
        expect(pills[1]!.textContent).to.contain('Execute Shell');
        expect(pills[2]!.textContent).to.contain('Failed Op');
      });

      it('should show check mark icon for complete status', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        const icon = pills[0]!.querySelector('.status-icon');
        expect(icon!.textContent).to.equal('\u2705');
      });

      it('should show hourglass icon for running status', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        const icon = pills[1]!.querySelector('.status-icon');
        expect(icon!.textContent).to.equal('\u23F3');
      });

      it('should show cross mark icon for error status', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        const icon = pills[2]!.querySelector('.status-icon');
        expect(icon!.textContent).to.equal('\u274C');
      });
    });

    describe('when toolCalls has an entry with unknown status', () => {
      let el: ChatMessageBubble;

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-unknown"
            .sender=${Sender.ASSISTANT}
            content="Unknown status"
            .toolCalls=${[{ toolCallId: 'tc-x', title: 'Mystery', status: 'pending' }]}
          ></chat-message-bubble>
        `);
      });

      it('should show bullet icon for unknown status', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-pill .status-icon');
        expect(icon!.textContent).to.equal('\u2022');
      });
    });
  });
});
