import { expect, fixture, html } from '@open-wc/testing';
import sinon from 'sinon';
import './confirmation-interlock-button.js';
import type { ConfirmationInterlockButton, TimerProvider } from './confirmation-interlock-button.js';

/**
 * Creates a controllable timer provider for testing.
 * Allows tests to manually trigger timer callbacks.
 */
function createTestTimerProvider(): TimerProvider & {
  tick: () => void;
  pendingCallback: (() => void) | null;
  wasCleared: boolean;
} {
  let pendingCallback: (() => void) | null = null;
  let wasCleared = false;

  return {
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    setTimeout: (callback: () => void, _delayMs: number): number => {
      pendingCallback = callback;
      wasCleared = false;
      return 1; // Return a dummy timer ID
    },
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    clearTimeout: (_id: number): void => {
      pendingCallback = null;
      wasCleared = true;
    },
    tick: () => {
      if (pendingCallback) {
        const cb = pendingCallback;
        pendingCallback = null;
        cb();
      }
    },
    get pendingCallback() {
      return pendingCallback;
    },
    get wasCleared() {
      return wasCleared;
    },
  };
}

describe('ConfirmationInterlockButton', () => {
  let el: ConfirmationInterlockButton;

  afterEach(() => {
    sinon.restore();
  });

  describe('when created with default properties', () => {
    beforeEach(async () => {
      el = await fixture(html`<confirmation-interlock-button></confirmation-interlock-button>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should have default label', () => {
      expect(el.label).to.equal('Confirm');
    });

    it('should have default confirmLabel', () => {
      expect(el.confirmLabel).to.equal('Are you sure?');
    });

    it('should have default yesLabel', () => {
      expect(el.yesLabel).to.equal('Yes');
    });

    it('should have default noLabel', () => {
      expect(el.noLabel).to.equal('No');
    });

    it('should not be armed initially', () => {
      expect(el.armed).to.be.false;
    });

    it('should not be disabled initially', () => {
      expect(el.disabled).to.be.false;
    });

    it('should have default disarmTimeoutMs', () => {
      expect(el.disarmTimeoutMs).to.equal(5000);
    });
  });

  describe('when in normal state', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button label="Change" .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
    });

    it('should render the trigger button', () => {
      const button = el.shadowRoot?.querySelector('button');
      expect(button).to.exist;
      expect(button?.textContent?.trim()).to.equal('Change');
    });

    it('should not render confirmation popup', () => {
      const confirmPopup = el.shadowRoot?.querySelector('.confirm-popup');
      expect(confirmPopup).to.not.exist;
    });
  });

  describe('when trigger button is clicked', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button label="Delete" .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
      const button = el.shadowRoot?.querySelector('button');
      button?.click();
      await el.updateComplete;
    });

    it('should become armed', () => {
      expect(el.armed).to.be.true;
    });

    it('should still render trigger button', () => {
      const buttons = el.shadowRoot?.querySelectorAll('button');
      const triggerButton = Array.from(buttons || []).find(
        btn => btn.textContent?.trim() === 'Delete'
      );
      expect(triggerButton).to.exist;
    });

    it('should render confirmation popup', () => {
      const confirmPopup = el.shadowRoot?.querySelector('.confirm-popup');
      expect(confirmPopup).to.exist;
    });

    it('should render confirm label', () => {
      const label = el.shadowRoot?.querySelector('.confirm-label');
      expect(label?.textContent).to.equal('Are you sure?');
    });

    it('should render yes button', () => {
      const yesButton = el.shadowRoot?.querySelector('.button-yes');
      expect(yesButton).to.exist;
      expect(yesButton?.textContent?.trim()).to.equal('Yes');
    });

    it('should render no button', () => {
      const noButton = el.shadowRoot?.querySelector('.button-no');
      expect(noButton).to.exist;
      expect(noButton?.textContent?.trim()).to.equal('No');
    });
  });

  describe('when custom labels are provided', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button
          label="Reset"
          confirmLabel="Clear all data?"
          yesLabel="Clear"
          noLabel="Cancel"
          .disarmTimeoutMs=${0}
        ></confirmation-interlock-button>
      `);
      el.arm();
      await el.updateComplete;
    });

    it('should display custom confirm label', () => {
      const label = el.shadowRoot?.querySelector('.confirm-label');
      expect(label?.textContent).to.equal('Clear all data?');
    });

    it('should display custom yes label', () => {
      const yesButton = el.shadowRoot?.querySelector('.button-yes');
      expect(yesButton?.textContent?.trim()).to.equal('Clear');
    });

    it('should display custom no label', () => {
      const noButton = el.shadowRoot?.querySelector('.button-no');
      expect(noButton?.textContent?.trim()).to.equal('Cancel');
    });
  });

  describe('when yes button is clicked', () => {
    let confirmedHandler: sinon.SinonStub;

    beforeEach(async () => {
      confirmedHandler = sinon.stub();
      el = await fixture(html`
        <confirmation-interlock-button
          .disarmTimeoutMs=${0}
          @confirmed=${confirmedHandler}
        ></confirmation-interlock-button>
      `);
      el.arm();
      await el.updateComplete;

      const yesButton = el.shadowRoot?.querySelector('.button-yes') as HTMLButtonElement;
      yesButton.click();
      await el.updateComplete;
    });

    it('should dispatch confirmed event', () => {
      expect(confirmedHandler).to.have.been.calledOnce;
    });

    it('should disarm the button', () => {
      expect(el.armed).to.be.false;
    });

    it('should close the popup', () => {
      const confirmPopup = el.shadowRoot?.querySelector('.confirm-popup');
      expect(confirmPopup).to.not.exist;
    });
  });

  describe('when no button is clicked', () => {
    let cancelledHandler: sinon.SinonStub;

    beforeEach(async () => {
      cancelledHandler = sinon.stub();
      el = await fixture(html`
        <confirmation-interlock-button
          .disarmTimeoutMs=${0}
          @cancelled=${cancelledHandler}
        ></confirmation-interlock-button>
      `);
      el.arm();
      await el.updateComplete;

      const noButton = el.shadowRoot?.querySelector('.button-no') as HTMLButtonElement;
      noButton.click();
      await el.updateComplete;
    });

    it('should dispatch cancelled event', () => {
      expect(cancelledHandler).to.have.been.calledOnce;
    });

    it('should disarm the button', () => {
      expect(el.armed).to.be.false;
    });

    it('should close the popup', () => {
      const confirmPopup = el.shadowRoot?.querySelector('.confirm-popup');
      expect(confirmPopup).to.not.exist;
    });
  });

  describe('when disabled', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button disabled .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
    });

    it('should have disabled trigger button', () => {
      const button = el.shadowRoot?.querySelector('button');
      expect(button?.disabled).to.be.true;
    });

    it('should not arm when arm() is called', () => {
      el.arm();
      expect(el.armed).to.be.false;
    });
  });

  describe('when disabled while armed', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
      el.arm();
      el.disabled = true;
      await el.updateComplete;
    });

    it('should have disabled yes button', () => {
      const yesButton = el.shadowRoot?.querySelector('.button-yes') as HTMLButtonElement;
      expect(yesButton?.disabled).to.be.true;
    });

    it('should have disabled no button', () => {
      const noButton = el.shadowRoot?.querySelector('.button-no') as HTMLButtonElement;
      expect(noButton?.disabled).to.be.true;
    });
  });

  describe('auto-disarm timeout', () => {
    describe('when armed with timeout', () => {
      let testTimer: ReturnType<typeof createTestTimerProvider>;

      beforeEach(async () => {
        testTimer = createTestTimerProvider();
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${2000}
            .timerProvider=${testTimer}
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should be armed before timer fires', () => {
        expect(el.armed).to.be.true;
        expect(testTimer.pendingCallback).to.exist;
      });

      it('should auto-disarm when timer fires', () => {
        testTimer.tick();
        expect(el.armed).to.be.false;
      });
    });

    describe('when disarmTimeoutMs is 0', () => {
      let testTimer: ReturnType<typeof createTestTimerProvider>;

      beforeEach(async () => {
        testTimer = createTestTimerProvider();
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${0}
            .timerProvider=${testTimer}
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should not schedule a timer', () => {
        expect(testTimer.pendingCallback).to.be.null;
      });

      it('should remain armed indefinitely', () => {
        expect(el.armed).to.be.true;
      });
    });

    describe('when yes is clicked before timeout', () => {
      let testTimer: ReturnType<typeof createTestTimerProvider>;
      let confirmedHandler: sinon.SinonStub;

      beforeEach(async () => {
        testTimer = createTestTimerProvider();
        confirmedHandler = sinon.stub();
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${2000}
            .timerProvider=${testTimer}
            @confirmed=${confirmedHandler}
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;

        const yesButton = el.shadowRoot?.querySelector('.button-yes') as HTMLButtonElement;
        yesButton.click();
        await el.updateComplete;
      });

      it('should dispatch confirmed event', () => {
        expect(confirmedHandler).to.have.been.calledOnce;
      });

      it('should clear the timer', () => {
        expect(testTimer.wasCleared).to.be.true;
        expect(testTimer.pendingCallback).to.be.null;
      });
    });
  });

  describe('arm() method', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
    });

    it('should set armed to true', () => {
      el.arm();
      expect(el.armed).to.be.true;
    });
  });

  describe('disarm() method', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
      `);
      el.arm();
    });

    it('should set armed to false', () => {
      el.disarm();
      expect(el.armed).to.be.false;
    });
  });

  describe('when disconnected from DOM', () => {
    describe('when armed element is removed', () => {
      let testTimer: ReturnType<typeof createTestTimerProvider>;

      beforeEach(async () => {
        testTimer = createTestTimerProvider();
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${2000}
            .timerProvider=${testTimer}
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;

        // Remove from DOM
        el.remove();
      });

      it('should clear disarm timer', () => {
        expect(testTimer.wasCleared).to.be.true;
      });

      it('should remain armed since disconnected', () => {
        // State unchanged since disconnected - timer was cleared before it could fire
        expect(el.armed).to.be.true;
      });
    });
  });

  describe('event bubbling', () => {
    describe('when confirmed event is dispatched', () => {
      let confirmedEvent: CustomEvent | null;

      beforeEach(async () => {
        confirmedEvent = null;
        el = await fixture(html`
          <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
        `);

        // Listen directly on the element for the composed event
        el.addEventListener('confirmed', (e: Event) => {
          confirmedEvent = e as CustomEvent;
        });

        el.arm();
        await el.updateComplete;

        const yesButton = el.shadowRoot?.querySelector('.button-yes') as HTMLButtonElement;
        yesButton.click();
      });

      it('should dispatch confirmed event', () => {
        expect(confirmedEvent).to.exist;
      });

      it('should be composed', () => {
        expect(confirmedEvent?.composed).to.be.true;
      });

      it('should bubble', () => {
        expect(confirmedEvent?.bubbles).to.be.true;
      });
    });
  });

  describe('popup positioning', () => {
    describe('when popupPosition is left', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${0}
            popupPosition="left"
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should position popup on left', () => {
        const popup = el.shadowRoot?.querySelector('.confirm-popup');
        expect(popup?.classList.contains('position-left')).to.be.true;
      });
    });

    describe('when popupPosition is right', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${0}
            popupPosition="right"
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should position popup on right', () => {
        const popup = el.shadowRoot?.querySelector('.confirm-popup');
        expect(popup?.classList.contains('position-right')).to.be.true;
      });
    });

    describe('when popupPosition is auto', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <confirmation-interlock-button
            .disarmTimeoutMs=${0}
            popupPosition="auto"
          ></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should have a position class', () => {
        const popup = el.shadowRoot?.querySelector('.confirm-popup');
        const hasPositionClass = popup?.classList.contains('position-left') ||
                                  popup?.classList.contains('position-right');
        expect(hasPositionClass).to.be.true;
      });
    });
  });

  describe('click outside behavior', () => {
    describe('when clicking the backdrop', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;

        // Trigger pointerdown on the backdrop (used instead of click for better mobile support)
        const backdrop = el.shadowRoot?.querySelector('.confirm-backdrop') as HTMLElement;
        backdrop?.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        await el.updateComplete;
      });

      it('should disarm the button', () => {
        expect(el.armed).to.be.false;
      });

      it('should close the popup', () => {
        const popup = el.shadowRoot?.querySelector('.confirm-popup');
        expect(popup).to.not.exist;
      });
    });

    describe('when popup is armed', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <confirmation-interlock-button .disarmTimeoutMs=${0}></confirmation-interlock-button>
        `);
        el.arm();
        await el.updateComplete;
      });

      it('should render backdrop', () => {
        const backdrop = el.shadowRoot?.querySelector('.confirm-backdrop');
        expect(backdrop).to.exist;
      });
    });
  });
});
