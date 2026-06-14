import { expect, fixture, html } from '@open-wc/testing';
import { useFakeTimers, type SinonFakeTimers } from 'sinon';
import { Sender } from '../gen/api/v1/chat_pb.js';
import './chat-message-bubble.js';
import type { ChatMessageBubble, ReactionGroup, ScrollToMessageEventDetail } from './chat-message-bubble.js';
import type { ToolCallState, PlanEntryState } from './page-chat-panel.js';

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

    describe('when a tool call has in_progress status (live)', () => {
      let el: ChatMessageBubble;
      const startedAtMs = Date.now() - 3500;
      const inProgressToolCall: ToolCallState = {
        toolCallId: 'tc-live',
        title: 'Reading a file',
        status: 'in_progress',
        kind: 'read',
        detail: '/some/path/to/file.ts',
        startedAtMs,
      };

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-live"
            .sender=${Sender.ASSISTANT}
            content="Using tools"
            .toolCalls=${[inProgressToolCall]}
          ></chat-message-bubble>
        `);
      });

      it('should render the live tool-call row', () => {
        const live = el.shadowRoot!.querySelector('.tool-call-live');
        expect(live).to.not.be.null;
      });

      it('should display the tool call title in the live row', () => {
        const title = el.shadowRoot!.querySelector('.tool-call-live-title');
        expect(title).to.not.be.null;
        expect(title!.textContent).to.equal('Reading a file');
      });

      it('should display the detail text', () => {
        const detail = el.shadowRoot!.querySelector('.tool-call-detail');
        expect(detail).to.not.be.null;
        expect(detail!.textContent).to.contain('/some/path/to/file.ts');
      });

      it('should display an elapsed time', () => {
        const elapsed = el.shadowRoot!.querySelector('.tool-call-elapsed');
        expect(elapsed).to.not.be.null;
        expect(elapsed!.textContent!.trim().length).to.be.greaterThan(0);
      });

      it('should show the hourglass icon for in_progress', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-live .status-icon');
        expect(icon!.textContent).to.equal('⏳');
      });

      it('should NOT render as a compact pill', () => {
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill).to.be.null;
      });
    });

    describe('when a tool call has pending status (live)', () => {
      let el: ChatMessageBubble;
      const pendingToolCall: ToolCallState = {
        toolCallId: 'tc-pending',
        title: 'Searching',
        status: 'pending',
        kind: 'search',
        detail: '',
        startedAtMs: Date.now(),
      };

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-pending"
            .sender=${Sender.ASSISTANT}
            content="Using tools"
            .toolCalls=${[pendingToolCall]}
          ></chat-message-bubble>
        `);
      });

      it('should render the live tool-call row', () => {
        const live = el.shadowRoot!.querySelector('.tool-call-live');
        expect(live).to.not.be.null;
      });

      it('should show the bullet dot icon for pending', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-live .status-icon');
        expect(icon!.textContent).to.equal('•');
      });
    });

    describe('when a tool call has completed status', () => {
      let el: ChatMessageBubble;
      const completedToolCall: ToolCallState = {
        toolCallId: 'tc-done',
        title: 'Read File',
        status: 'completed',
        kind: 'read',
        detail: '/path/file.ts',
        startedAtMs: Date.now() - 2000,
      };

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-done"
            .sender=${Sender.ASSISTANT}
            content="Done"
            .toolCalls=${[completedToolCall]}
          ></chat-message-bubble>
        `);
      });

      it('should render as a compact pill', () => {
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill).to.not.be.null;
      });

      it('should show the check mark icon for completed', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-pill .status-icon');
        expect(icon!.textContent).to.equal('✅');
      });

      it('should display the title in the pill', () => {
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill!.textContent).to.contain('Read File');
      });

      it('should display the detail in the pill (specifics, not just the category)', () => {
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill!.textContent).to.contain('/path/file.ts');
      });

      it('should NOT render the expanded live row', () => {
        const live = el.shadowRoot!.querySelector('.tool-call-live');
        expect(live).to.be.null;
      });

      it('should NOT render the detail element', () => {
        const detail = el.shadowRoot!.querySelector('.tool-call-detail');
        expect(detail).to.be.null;
      });
    });

    describe('when a tool call has failed status', () => {
      let el: ChatMessageBubble;
      const failedToolCall: ToolCallState = {
        toolCallId: 'tc-fail',
        title: 'Execute Shell',
        status: 'failed',
        kind: 'execute',
        detail: 'Exit code 1',
        startedAtMs: Date.now() - 1000,
      };

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-tc-fail"
            .sender=${Sender.ASSISTANT}
            content="Failed"
            .toolCalls=${[failedToolCall]}
          ></chat-message-bubble>
        `);
      });

      it('should render as a compact pill', () => {
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill).to.not.be.null;
      });

      it('should show the cross mark icon for failed', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-pill .status-icon');
        expect(icon!.textContent).to.equal('❌');
      });

      it('should NOT render the expanded live row', () => {
        const live = el.shadowRoot!.querySelector('.tool-call-live');
        expect(live).to.be.null;
      });
    });

    describe('when toolCalls has entries with mixed statuses', () => {
      let el: ChatMessageBubble;
      const toolCalls: ToolCallState[] = [
        { toolCallId: 'tc-1', title: 'Read File', status: 'completed', kind: 'read', detail: '', startedAtMs: Date.now() - 5000 },
        { toolCallId: 'tc-2', title: 'Execute Shell', status: 'in_progress', kind: 'execute', detail: 'running...', startedAtMs: Date.now() - 1000 },
        { toolCallId: 'tc-3', title: 'Failed Op', status: 'failed', kind: 'other', detail: '', startedAtMs: Date.now() - 500 },
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

      it('should render one pill for the completed tool call', () => {
        const pills = el.shadowRoot!.querySelectorAll('.tool-call-pill');
        expect(pills.length).to.equal(2);
      });

      it('should render one live row for the in_progress tool call', () => {
        const liveRows = el.shadowRoot!.querySelectorAll('.tool-call-live');
        expect(liveRows.length).to.equal(1);
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
            .toolCalls=${[{ toolCallId: 'tc-x', title: 'Mystery', status: 'unknown_status', kind: '', detail: '', startedAtMs: Date.now() }]}
          ></chat-message-bubble>
        `);
      });

      it('should render as a compact pill (not live) for unknown status', () => {
        // Unknown status is not pending/in_progress, so it renders as a compact pill
        const pill = el.shadowRoot!.querySelector('.tool-call-pill');
        expect(pill).to.not.be.null;
      });

      it('should show bullet icon for unknown status', () => {
        const icon = el.shadowRoot!.querySelector('.tool-call-pill .status-icon');
        expect(icon!.textContent).to.equal('•');
      });
    });
  });

  describe('elapsed timer lifecycle', () => {
    let clock: SinonFakeTimers;

    beforeEach(() => {
      clock = useFakeTimers();
    });

    afterEach(() => {
      clock.restore();
    });

    describe('when connected with a live tool call', () => {
      let el: ChatMessageBubble;
      const liveToolCall: ToolCallState = {
        toolCallId: 'tc-timer',
        title: 'Running',
        status: 'in_progress',
        kind: 'execute',
        detail: 'working...',
        startedAtMs: 0,
      };

      let elapsedBefore: string | null;
      let elapsedAfter: string | null;

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-timer"
            .sender=${Sender.ASSISTANT}
            content="Tool running"
            .toolCalls=${[liveToolCall]}
          ></chat-message-bubble>
        `);
        await el.updateComplete;
        elapsedBefore = el.shadowRoot!.querySelector('.tool-call-elapsed')!.textContent;
        clock.tick(1000);
        await el.updateComplete;
        elapsedAfter = el.shadowRoot!.querySelector('.tool-call-elapsed')!.textContent;
      });

      it('should render an elapsed indicator for the live tool call', () => {
        expect(elapsedBefore).to.not.be.null;
      });

      it('should advance the elapsed time when the timer ticks', () => {
        expect(elapsedAfter).to.not.equal(elapsedBefore);
      });

      describe('when the tool call completes (status changes)', () => {
        beforeEach(async () => {
          el.toolCalls = [{ ...liveToolCall, status: 'completed' }];
          await el.updateComplete;
        });

        it('should not render the live row anymore', () => {
          const live = el.shadowRoot!.querySelector('.tool-call-live');
          expect(live).to.be.null;
        });
      });
    });

    describe('when a live tool call has no startedAtMs (historical, not replayed with a timestamp)', () => {
      let el: ChatMessageBubble;

      beforeEach(async () => {
        const toolCallWithoutStart = {
          toolCallId: 'tc-no-start',
          title: 'Running',
          status: 'in_progress',
          kind: 'execute',
          detail: 'working...',
          startedAtMs: undefined as unknown as number,
        };
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-no-start"
            .sender=${Sender.ASSISTANT}
            content="Tool running"
            .toolCalls=${[toolCallWithoutStart]}
          ></chat-message-bubble>
        `);
        await el.updateComplete;
      });

      it('should still render the live row', () => {
        expect(el.shadowRoot!.querySelector('.tool-call-live')).to.not.be.null;
      });

      it('should not render an elapsed indicator (avoids "NaNs")', () => {
        expect(el.shadowRoot!.querySelector('.tool-call-elapsed')).to.be.null;
      });
    });

    describe('when connected with only completed tool calls', () => {
      let el: ChatMessageBubble;
      const completedToolCall: ToolCallState = {
        toolCallId: 'tc-done',
        title: 'Done',
        status: 'completed',
        kind: 'read',
        detail: '',
        startedAtMs: 0,
      };

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-timer-done"
            .sender=${Sender.ASSISTANT}
            content="Tool done"
            .toolCalls=${[completedToolCall]}
          ></chat-message-bubble>
        `);
        await el.updateComplete;
      });

      it('should not show a live row', () => {
        const live = el.shadowRoot!.querySelector('.tool-call-live');
        expect(live).to.be.null;
      });
    });
  });

  describe('plan rendering', () => {

    describe('when plan is empty', () => {
      let el: ChatMessageBubble;

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-plan-empty"
            .sender=${Sender.ASSISTANT}
            content="No plan"
            .plan=${[]}
          ></chat-message-bubble>
        `);
      });

      it('should not render the plan block', () => {
        const planBlock = el.shadowRoot!.querySelector('.plan-block');
        expect(planBlock).to.be.null;
      });
    });

    describe('when plan is set to undefined (message state without a plan field)', () => {
      let el: ChatMessageBubble;

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-plan-undefined"
            .sender=${Sender.ASSISTANT}
            content="No plan field"
            .plan=${undefined as unknown as PlanEntryState[]}
          ></chat-message-bubble>
        `);
      });

      it('should render without throwing', () => {
        expect(el).to.exist;
      });

      it('should not render the plan block', () => {
        const planBlock = el.shadowRoot!.querySelector('.plan-block');
        expect(planBlock).to.be.null;
      });
    });

    describe('when plan has entries', () => {
      let el: ChatMessageBubble;
      const plan: PlanEntryState[] = [
        { content: 'Search for the page', status: 'completed', priority: 'high' },
        { content: 'Read the content', status: 'in_progress', priority: 'high' },
        { content: 'Update the page', status: 'pending', priority: 'medium' },
      ];

      beforeEach(async () => {
        el = await fixture(html`
          <chat-message-bubble
            message-id="msg-plan"
            .sender=${Sender.ASSISTANT}
            content="Working..."
            .plan=${plan}
          ></chat-message-bubble>
        `);
      });

      it('should render the plan block', () => {
        const planBlock = el.shadowRoot!.querySelector('.plan-block');
        expect(planBlock).to.not.be.null;
      });

      it('should render three plan entries', () => {
        const entries = el.shadowRoot!.querySelectorAll('.plan-entry');
        expect(entries.length).to.equal(3);
      });

      it('should show the check glyph for completed entries', () => {
        const entries = el.shadowRoot!.querySelectorAll('.plan-entry');
        const icon = entries[0]!.querySelector('.plan-entry-icon');
        expect(icon!.textContent).to.equal('☑');
      });

      it('should show the spinning glyph for in_progress entries', () => {
        const entries = el.shadowRoot!.querySelectorAll('.plan-entry');
        const icon = entries[1]!.querySelector('.plan-entry-icon');
        expect(icon!.textContent).to.equal('🔄');
      });

      it('should show the empty checkbox glyph for pending entries', () => {
        const entries = el.shadowRoot!.querySelectorAll('.plan-entry');
        const icon = entries[2]!.querySelector('.plan-entry-icon');
        expect(icon!.textContent).to.equal('☐');
      });

      it('should display the content of each entry', () => {
        const entries = el.shadowRoot!.querySelectorAll('.plan-entry');
        expect(entries[0]!.textContent).to.contain('Search for the page');
        expect(entries[1]!.textContent).to.contain('Read the content');
        expect(entries[2]!.textContent).to.contain('Update the page');
      });
    });
  });
});
