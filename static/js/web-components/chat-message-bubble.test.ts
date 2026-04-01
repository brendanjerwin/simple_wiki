import { expect, fixture, html } from '@open-wc/testing';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './chat-message-bubble.js';
import type { ChatMessageBubble, ReactionGroup, ScrollToMessageEventDetail } from './chat-message-bubble.js';

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
      expect(bubble!.classList.contains('user')).to.equal(true);
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
      expect(bubble!.classList.contains('assistant')).to.equal(true);
    });

    it('should render HTML content', () => {
      const strong = el.shadowRoot!.querySelector('.content strong');
      expect(strong).to.not.equal(null);
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
      expect(edited).to.not.equal(null);
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
      expect(edited).to.equal(null);
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
      expect(reactions).to.equal(null);
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
      expect(replyLink).to.not.equal(null);
    });

    describe('when the reply link is clicked', () => {
      beforeEach(async () => {
        const replyLink = el.shadowRoot!.querySelector('.reply-link')!;
        (replyLink as HTMLElement).click();
      });

      it('should dispatch scroll-to-message event', () => {
        expect(eventDetail).to.not.equal(null);
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
      expect(replyLink).to.equal(null);
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
      expect(name).to.not.equal(null);
      expect(name!.textContent).to.equal('Alice');
    });
  });
});
