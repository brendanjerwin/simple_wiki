import { html, fixture, expect, assert } from '@open-wc/testing';
import { stub, useFakeTimers, type SinonFakeTimers, type SinonStub } from 'sinon';
import { ConnectError, Code } from '@connectrpc/connect';
import { ErrorDisplay, type ErrorAction } from './error-display.js';
import { AugmentErrorService, AugmentedError, ErrorKind } from './augment-error-service.js';

function timeout(ms: number, message: string): Promise<never> {
  return new Promise((_, reject) =>
    setTimeout(() => reject(new Error(message)), ms),
  );
}

describe('ErrorDisplay', () => {
  let el: ErrorDisplay;
  let fetchStub: ReturnType<typeof stub>;
  let originalClipboard: PropertyDescriptor | undefined;

  beforeEach(async () => {
    // Prevent any potential network calls that could cause hanging
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response('{}'));
    originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    
    el = await Promise.race([
      fixture<ErrorDisplay>(html`<error-display></error-display>`),
      timeout(5000, "ErrorDisplay fixture timed out"),
    ]);
  });

  afterEach(() => {
    if (fetchStub) {
      fetchStub.restore();
    }

    if (originalClipboard) {
      Object.defineProperty(navigator, 'clipboard', originalClipboard);
    } else {
      Reflect.deleteProperty(navigator, 'clipboard');
    }
  });

  it('should exist', () => {
    assert.instanceOf(el, ErrorDisplay);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName).to.equal('ERROR-DISPLAY');
  });

  it('should display error message', async () => {
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const messageElement = el.shadowRoot?.querySelector('.error-message');
    expect(messageElement?.textContent?.trim()).to.equal('Test error message');
  });

  it('should display icon', async () => {
    const originalError = new Error('Test error message');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const iconElement = el.shadowRoot?.querySelector('.error-icon');
    expect(iconElement?.textContent).to.equal('❌');
  });

  it('should show expand button when stack exists', async () => {
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const expandButton = el.shadowRoot?.querySelector('.expand-button');
    expect(expandButton).to.exist;
  });

  it('should expand when button clicked', async () => {
    const originalError = new Error('Test error message');
    originalError.stack = 'Error: Test error message\n    at Object.<anonymous> (test.js:1:1)';
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.ERROR,
      'error'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
    expandButton?.click();
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update after click timed out"),
    ]);

    const detailsElement = el.shadowRoot?.querySelector('.error-details');
    expect(detailsElement?.getAttribute('aria-hidden')).to.equal('false');
  });

  it('should display structured message with goal', async () => {
    const originalError = new Error('Connection refused');
    const augmentedError = new AugmentedError(
      originalError,
      ErrorKind.NETWORK,
      'network',
      'saving document'
    );
    el.augmentedError = augmentedError;
    await Promise.race([
      el.updateComplete,
      timeout(5000, "Component update timed out"),
    ]);

    const goalElement = el.shadowRoot?.querySelector('.error-goal');
    const detailElement = el.shadowRoot?.querySelector('.error-detail');
    
    expect(goalElement?.textContent?.trim()).to.equal('Error while saving document:');
    expect(detailElement?.textContent?.trim()).to.equal('Connection refused');
  });

  describe('action button', () => {
    describe('when action is not provided', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');
        delete el.action;
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should not display action button', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton).to.not.exist;
      });

      it('should not display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.not.exist;
      });
    });

    describe('when action is provided', () => {
      let onClickStub: SinonStub;
      let action: ErrorAction;

      beforeEach(async () => {
        onClickStub = stub();
        action = {
          label: 'Try Again',
          onClick: onClickStub
        };

        const originalError = new Error('Test error');
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');
        el.action = action;
        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display action button', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton).to.exist;
      });

      it('should display action label', () => {
        const actionButton = el.shadowRoot?.querySelector('.action-button');
        expect(actionButton?.textContent?.trim()).to.equal('Try Again');
      });

      it('should display actions container', () => {
        const actionsContainer = el.shadowRoot?.querySelector('.error-actions');
        expect(actionsContainer).to.exist;
      });

      describe('when action button is clicked', () => {
        beforeEach(async () => {
          const actionButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.action-button');
          actionButton?.click();
          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should call onClick callback', () => {
          expect(onClickStub).to.have.been.calledOnce;
        });
      });
    });
  });

  describe('_renderDetails', () => {
    describe('when error has no stack', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        delete originalError.stack;
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should not display expand button', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton).to.equal(null);
      });

      it('should not display details section', () => {
        const detailsSection = el.shadowRoot?.querySelector('.error-details');
        expect(detailsSection).to.equal(null);
      });
    });

    describe('when error has stack', () => {
      beforeEach(async () => {
        const originalError = new Error('Test error');
        originalError.stack = 'Error: Test error\n    at Object.<anonymous> (test.js:1:1)';
        el.augmentedError = new AugmentedError(originalError, ErrorKind.ERROR, 'error');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should display expand button', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton).to.not.equal(null);
      });

      it('should show "Show details" label initially', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.textContent?.trim()).to.include('Show details');
      });

      it('should have details hidden initially', () => {
        const detailsSection = el.shadowRoot?.querySelector('.error-details');
        expect(detailsSection?.getAttribute('aria-hidden')).to.equal('true');
      });

      it('should set aria-expanded to false on button initially', () => {
        const expandButton = el.shadowRoot?.querySelector('.expand-button');
        expect(expandButton?.getAttribute('aria-expanded')).to.equal('false');
      });

      describe('when expand button is clicked', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.click();

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update after click timed out"),
          ]);
        });

        it('should show details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });

        it('should change label to "Hide details"', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.textContent?.trim()).to.include('Hide details');
        });

        it('should set aria-expanded to true on button', () => {
          const expandButton = el.shadowRoot?.querySelector('.expand-button');
          expect(expandButton?.getAttribute('aria-expanded')).to.equal('true');
        });

        it('should apply expanded class to expand icon', () => {
          const expandIcon = el.shadowRoot?.querySelector('.expand-icon');
          expect(expandIcon?.classList.contains('expanded')).to.equal(true);
        });

        describe('when expand button is clicked again', () => {
          beforeEach(async () => {
            const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
            expandButton?.click();

            await Promise.race([
              el.updateComplete,
              timeout(5000, "Component update after second click timed out"),
            ]);
          });

          it('should hide details again', () => {
            const detailsSection = el.shadowRoot?.querySelector('.error-details');
            expect(detailsSection?.getAttribute('aria-hidden')).to.equal('true');
          });

          it('should revert label to "Show details"', () => {
            const expandButton = el.shadowRoot?.querySelector('.expand-button');
            expect(expandButton?.textContent?.trim()).to.include('Show details');
          });
        });
      });

      describe('when Enter key is pressed on expand button', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should expand details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });
      });

      describe('when Space key is pressed on expand button', () => {
        beforeEach(async () => {
          const expandButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.expand-button');
          expandButton?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));

          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should expand details', () => {
          const detailsSection = el.shadowRoot?.querySelector('.error-details');
          expect(detailsSection?.getAttribute('aria-hidden')).to.equal('false');
        });
      });
    });
  });

  describe('copy details button', () => {
    let writeTextStub: SinonStub;
    let copiedMarkdown: string;
    let clock: SinonFakeTimers;

    beforeEach(() => {
      writeTextStub = stub().resolves();
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: {
          writeText: writeTextStub,
        },
      });
    });

    afterEach(() => {
      clock?.restore();
    });

    describe('when the error is a bare Error', () => {
      beforeEach(async () => {
        clock = useFakeTimers({
          now: new Date('2026-05-23T12:34:56.000Z'),
          toFake: ['Date', 'setTimeout', 'clearTimeout'],
        });
        const error = new Error('Bare failure');
        error.stack = 'Error: Bare failure\n    at test.ts:1:1';
        el.augmentedError = AugmentErrorService.augmentError(error, 'load dashboard');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);

        const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
        copyButton?.click();

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
        copiedMarkdown = writeTextStub.firstCall.args[0] as string;
      });

      it('should write markdown details to the clipboard', () => {
        expect(writeTextStub.calledOnce).to.equal(true);
        expect(copiedMarkdown).to.include('# Error Details');
      });

      it('should include the visible message', () => {
        expect(copiedMarkdown).to.include('**Message:** Bare failure');
      });

      it('should include the failed goal', () => {
        expect(copiedMarkdown).to.include('**Failed goal:** load dashboard');
      });

      it('should include the stack trace', () => {
        expect(copiedMarkdown).to.include('Error: Bare failure');
      });

      it('should include browser context', () => {
        expect(copiedMarkdown).to.include('**Timestamp:** 2026-05-23T12:34:56.000Z');
        expect(copiedMarkdown).to.include(`**Current URL:** ${window.location.href}`);
        expect(copiedMarkdown).to.include(`**User-Agent:** ${navigator.userAgent}`);
      });

      it('should show a copied confirmation', () => {
        const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
        expect(copyButton?.textContent?.trim()).to.equal('Copied!');
      });

      describe('when the confirmation timeout completes', () => {
        beforeEach(async () => {
          await clock.tickAsync(1500);
          await Promise.race([
            el.updateComplete,
            timeout(5000, "Component update timed out"),
          ]);
        });

        it('should restore the button label', () => {
          const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
          expect(copyButton?.textContent?.trim()).to.equal('Copy details');
        });
      });
    });

    describe('when the error is a ConnectError with metadata', () => {
      beforeEach(async () => {
        const error = new ConnectError(
          'Permission denied',
          Code.PermissionDenied,
          {
            'x-request-id': 'request-123',
            'x-attempt-number': '2',
          }
        );
        error.stack = 'ConnectError: Permission denied\n    at rpc.ts:2:1';
        el.augmentedError = AugmentErrorService.augmentError(error, 'sync tasks');

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);

        const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
        copyButton?.click();

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
        copiedMarkdown = writeTextStub.firstCall.args[0] as string;
      });

      it('should include the Connect code', () => {
        expect(copiedMarkdown).to.include('**Connect code:** PermissionDenied');
      });

      it('should include the gRPC code', () => {
        expect(copiedMarkdown).to.include('**gRPC code:** 7');
      });

      it('should include metadata', () => {
        expect(copiedMarkdown).to.include('- x-request-id: request-123');
        expect(copiedMarkdown).to.include('- x-attempt-number: 2');
      });
    });

    describe('when the error has a multi-link cause chain', () => {
      beforeEach(async () => {
        const root = new Error('socket closed');
        const middle = new Error('transport failed', { cause: root });
        const error = new Error('request failed', { cause: middle });
        el.augmentedError = AugmentErrorService.augmentError(error);

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);

        const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
        copyButton?.click();

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
        copiedMarkdown = writeTextStub.firstCall.args[0] as string;
      });

      it('should include each cause chain link', () => {
        expect(copiedMarkdown).to.include('1. Error: request failed');
        expect(copiedMarkdown).to.include('2. Error: transport failed');
        expect(copiedMarkdown).to.include('3. Error: socket closed');
      });

      it('should include empty diagnostic fields', () => {
        expect(copiedMarkdown).to.include('**Failed goal:** (none)');
        expect(copiedMarkdown).to.include('**Metadata:** (none)');
        expect(copiedMarkdown).to.include('**Wiki version:** (none)');
      });
    });

    describe('when the clipboard API is unavailable', () => {
      beforeEach(async () => {
        Object.defineProperty(navigator, 'clipboard', {
          configurable: true,
          value: undefined,
        });
        el.augmentedError = AugmentErrorService.augmentError(new Error('Clipboard unavailable'));

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);

        const copyButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.copy-details-button');
        copyButton?.click();

        await Promise.race([
          el.updateComplete,
          timeout(5000, "Component update timed out"),
        ]);
      });

      it('should render the fallback textarea', () => {
        const textarea = el.shadowRoot?.querySelector<HTMLTextAreaElement>('.copy-details-fallback');
        expect(textarea).not.to.equal(null);
        expect(textarea?.value).to.include('Clipboard unavailable');
      });

      it('should auto-select the fallback details', () => {
        const textarea = el.shadowRoot?.querySelector<HTMLTextAreaElement>('.copy-details-fallback');
        expect(el.shadowRoot?.activeElement).to.equal(textarea);
        expect(textarea?.selectionStart).to.equal(0);
        expect(textarea?.selectionEnd).to.equal(textarea?.value.length);
      });
    });
  });
});
