import { expect, fixture, html } from '@open-wc/testing';
import { LitElement } from 'lit';
import sinon from 'sinon';
import { handleKeydownFocusTrap, restoreFocus, NativeDialogMixin } from './native-dialog-mixin.js';

// ─── Minimal test component ──────────────────────────────────────────────────

class TestNativeDialog extends NativeDialogMixin(LitElement) {
  /** Expose protected field as public for test assertions. */
  get previouslyFocusedElement(): Element | null {
    return (this as unknown as { _previouslyFocusedElement: Element | null })._previouslyFocusedElement;
  }

  protected _closeDialog(): void {
    this.open = false;
  }

  override render() {
    return html`<dialog
      @cancel=${this._handleDialogCancel}
      @click=${this._handleDialogClick}
      @keydown=${this._handleKeydown}
    ></dialog>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'test-native-dialog': TestNativeDialog;
  }
}

if (!customElements.get('test-native-dialog')) {
  customElements.define('test-native-dialog', TestNativeDialog);
}

describe('handleKeydownFocusTrap', () => {
  let shadowRootMock: { querySelectorAll: sinon.SinonStub };
  let button1: HTMLButtonElement;
  let button2: HTMLButtonElement;
  let button3: HTMLButtonElement;

  beforeEach(() => {
    button1 = document.createElement('button');
    button2 = document.createElement('button');
    button3 = document.createElement('button');
    shadowRootMock = {
      querySelectorAll: sinon.stub(),
    };
  });

  it('should exist', () => {
    expect(handleKeydownFocusTrap).to.be.a('function');
  });

  describe('when a non-Tab key is pressed', () => {
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      shadowRootMock.querySelectorAll.returns([button1, button2]);
      preventDefaultSpy = sinon.spy();
      const event = new KeyboardEvent('keydown', { key: 'Enter', composed: true });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);
      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    it('should not call preventDefault', () => {
      expect(preventDefaultSpy).to.not.have.been.called;
    });
  });

  describe('when shadowRoot is null', () => {
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      preventDefaultSpy = sinon.spy();
      const event = new KeyboardEvent('keydown', { key: 'Tab', composed: true });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);
      handleKeydownFocusTrap(null, event);
    });

    it('should not call preventDefault', () => {
      expect(preventDefaultSpy).to.not.have.been.called;
    });
  });

  describe('when Tab is pressed and there are no focusable buttons', () => {
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      shadowRootMock.querySelectorAll.returns([]);
      preventDefaultSpy = sinon.spy();
      const event = new KeyboardEvent('keydown', { key: 'Tab', composed: true });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);
      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    it('should not call preventDefault', () => {
      expect(preventDefaultSpy).to.not.have.been.called;
    });
  });

  describe('when Tab is pressed from the first button', () => {
    let focusSpy2: sinon.SinonSpy;
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      document.body.appendChild(button1);
      document.body.appendChild(button2);
      shadowRootMock.querySelectorAll.returns([button1, button2]);
      focusSpy2 = sinon.spy(button2, 'focus');
      preventDefaultSpy = sinon.spy();

      const event = new KeyboardEvent('keydown', { key: 'Tab', composed: true });
      Object.defineProperty(event, 'composedPath', {
        value: () => [button1],
      });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);

      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    afterEach(() => {
      button1.remove();
      button2.remove();
    });

    it('should call preventDefault', () => {
      expect(preventDefaultSpy).to.have.been.calledOnce;
    });

    it('should move focus to the next button', () => {
      expect(focusSpy2).to.have.been.calledOnce;
    });
  });

  describe('when Tab is pressed from the last button', () => {
    let focusSpy1: sinon.SinonSpy;
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      document.body.appendChild(button1);
      document.body.appendChild(button2);
      shadowRootMock.querySelectorAll.returns([button1, button2]);
      focusSpy1 = sinon.spy(button1, 'focus');
      preventDefaultSpy = sinon.spy();

      const event = new KeyboardEvent('keydown', { key: 'Tab', composed: true });
      Object.defineProperty(event, 'composedPath', {
        value: () => [button2],
      });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);

      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    afterEach(() => {
      button1.remove();
      button2.remove();
    });

    it('should call preventDefault', () => {
      expect(preventDefaultSpy).to.have.been.calledOnce;
    });

    it('should wrap focus to the first button', () => {
      expect(focusSpy1).to.have.been.calledOnce;
    });
  });

  describe('when Shift+Tab is pressed from the last button', () => {
    let focusSpy1: sinon.SinonSpy;
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      document.body.appendChild(button1);
      document.body.appendChild(button2);
      document.body.appendChild(button3);
      shadowRootMock.querySelectorAll.returns([button1, button2, button3]);
      focusSpy1 = sinon.spy(button2, 'focus');
      preventDefaultSpy = sinon.spy();

      const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, composed: true });
      Object.defineProperty(event, 'composedPath', {
        value: () => [button3],
      });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);

      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    afterEach(() => {
      button1.remove();
      button2.remove();
      button3.remove();
    });

    it('should call preventDefault', () => {
      expect(preventDefaultSpy).to.have.been.calledOnce;
    });

    it('should move focus to the previous button', () => {
      expect(focusSpy1).to.have.been.calledOnce;
    });
  });

  describe('when Shift+Tab is pressed from the first button', () => {
    let focusSpy2: sinon.SinonSpy;
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(() => {
      document.body.appendChild(button1);
      document.body.appendChild(button2);
      shadowRootMock.querySelectorAll.returns([button1, button2]);
      focusSpy2 = sinon.spy(button2, 'focus');
      preventDefaultSpy = sinon.spy();

      const event = new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, composed: true });
      Object.defineProperty(event, 'composedPath', {
        value: () => [button1],
      });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);

      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    afterEach(() => {
      button1.remove();
      button2.remove();
    });

    it('should call preventDefault', () => {
      expect(preventDefaultSpy).to.have.been.calledOnce;
    });

    it('should wrap focus to the last button', () => {
      expect(focusSpy2).to.have.been.calledOnce;
    });
  });

  describe('when Tab is pressed but the target is not in the focusable list', () => {
    let focusSpy1: sinon.SinonSpy;
    let focusSpy2: sinon.SinonSpy;
    let preventDefaultSpy: sinon.SinonSpy;
    let outsideButton: HTMLButtonElement;

    beforeEach(() => {
      outsideButton = document.createElement('button');
      document.body.appendChild(button1);
      document.body.appendChild(button2);
      shadowRootMock.querySelectorAll.returns([button1, button2]);
      focusSpy1 = sinon.spy(button1, 'focus');
      focusSpy2 = sinon.spy(button2, 'focus');
      preventDefaultSpy = sinon.spy();

      const event = new KeyboardEvent('keydown', { key: 'Tab', composed: true });
      Object.defineProperty(event, 'composedPath', {
        value: () => [outsideButton],
      });
      sinon.stub(event, 'preventDefault').callsFake(preventDefaultSpy);

      handleKeydownFocusTrap(shadowRootMock as unknown as ShadowRoot, event);
    });

    afterEach(() => {
      button1.remove();
      button2.remove();
    });

    it('should not call preventDefault', () => {
      expect(preventDefaultSpy).to.not.have.been.called;
    });

    it('should not move focus to button1', () => {
      expect(focusSpy1).to.not.have.been.called;
    });

    it('should not move focus to button2', () => {
      expect(focusSpy2).to.not.have.been.called;
    });
  });
});

describe('restoreFocus', () => {
  it('should exist', () => {
    expect(restoreFocus).to.be.a('function');
  });

  describe('when target is null', () => {
    beforeEach(() => {
      restoreFocus(null);
    });

    it('should not throw', () => {
      expect(true).to.be.true;
    });
  });

  describe('when target is a visible focusable button', () => {
    let button: HTMLButtonElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      button = document.createElement('button');
      document.body.appendChild(button);
      focusSpy = sinon.spy(button, 'focus');
      restoreFocus(button);
    });

    afterEach(() => {
      button.remove();
    });

    it('should focus the button directly', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when target is a visible <a href> element', () => {
    let anchor: HTMLAnchorElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      anchor = document.createElement('a');
      anchor.href = '#test';
      document.body.appendChild(anchor);
      focusSpy = sinon.spy(anchor, 'focus');
      restoreFocus(anchor);
    });

    afterEach(() => {
      anchor.remove();
    });

    it('should focus the anchor directly', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when target is a visible [tabindex="0"] element', () => {
    let div: HTMLDivElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      div = document.createElement('div');
      div.tabIndex = 0;
      document.body.appendChild(div);
      focusSpy = sinon.spy(div, 'focus');
      restoreFocus(div);
    });

    afterEach(() => {
      div.remove();
    });

    it('should focus the element directly', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when target is a visible but non-focusable element', () => {
    let container: HTMLDivElement;
    let target: HTMLSpanElement;
    let sibling: HTMLButtonElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      container = document.createElement('div');
      target = document.createElement('span');
      sibling = document.createElement('button');
      container.appendChild(target);
      container.appendChild(sibling);
      document.body.appendChild(container);
      focusSpy = sinon.spy(sibling, 'focus');
      restoreFocus(target);
    });

    afterEach(() => {
      container.remove();
    });

    it('should walk up and focus the first rendered focusable in the ancestor subtree', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when target is hidden inside a display:none wrapper', () => {
    let outerContainer: HTMLDivElement;
    let hiddenWrapper: HTMLDivElement;
    let hiddenTarget: HTMLButtonElement;
    let visibleTrigger: HTMLButtonElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      outerContainer = document.createElement('div');
      visibleTrigger = document.createElement('button');
      hiddenWrapper = document.createElement('div');
      hiddenWrapper.style.display = 'none';
      hiddenTarget = document.createElement('button');
      hiddenWrapper.appendChild(hiddenTarget);
      outerContainer.appendChild(visibleTrigger);
      outerContainer.appendChild(hiddenWrapper);
      document.body.appendChild(outerContainer);
      focusSpy = sinon.spy(visibleTrigger, 'focus');
      restoreFocus(hiddenTarget);
    });

    afterEach(() => {
      outerContainer.remove();
    });

    it('should focus the nearest visible focusable element in the ancestor subtree', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });

  describe('when the target hidden wrapper has no visible focusable siblings', () => {
    let outerContainer: HTMLDivElement;
    let hiddenWrapper: HTMLDivElement;
    let hiddenTarget: HTMLButtonElement;
    let outerButton: HTMLButtonElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(() => {
      outerContainer = document.createElement('div');
      hiddenWrapper = document.createElement('div');
      hiddenWrapper.style.display = 'none';
      hiddenTarget = document.createElement('button');
      hiddenWrapper.appendChild(hiddenTarget);
      outerContainer.appendChild(hiddenWrapper);
      // Parent of outerContainer has a visible button
      outerButton = document.createElement('button');
      const grandparent = document.createElement('div');
      grandparent.appendChild(outerButton);
      grandparent.appendChild(outerContainer);
      document.body.appendChild(grandparent);
      focusSpy = sinon.spy(outerButton, 'focus');
      restoreFocus(hiddenTarget);
    });

    afterEach(() => {
      outerButton.closest('div')?.remove();
    });

    it('should walk further up the DOM tree and focus the grandparent-level button', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });
  });
});

describe('NativeDialogMixin', () => {
  describe('when open is set to true', () => {
    let el: TestNativeDialog;
    let triggerButton: HTMLButtonElement;

    beforeEach(async () => {
      triggerButton = document.createElement('button');
      document.body.appendChild(triggerButton);
      triggerButton.focus();

      el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
      el.open = true;
      await el.updateComplete;
    });

    afterEach(() => {
      triggerButton.remove();
      el.open = false;
    });

    it('should capture the previously focused element', () => {
      expect(el.previouslyFocusedElement).to.equal(triggerButton);
    });

    it('should open the dialog', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.open).to.be.true;
    });
  });

  describe('when open transitions from true to false', () => {
    let el: TestNativeDialog;
    let triggerButton: HTMLButtonElement;
    let focusSpy: sinon.SinonSpy;

    beforeEach(async () => {
      triggerButton = document.createElement('button');
      document.body.appendChild(triggerButton);
      triggerButton.focus();

      el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
      el.open = true;
      await el.updateComplete;

      focusSpy = sinon.spy(triggerButton, 'focus');
      el.open = false;
      await el.updateComplete;
    });

    afterEach(() => {
      triggerButton.remove();
    });

    it('should restore focus to the previously focused element', () => {
      expect(focusSpy).to.have.been.calledOnce;
    });

    it('should clear _previouslyFocusedElement', () => {
      expect(el.previouslyFocusedElement).to.be.null;
    });

    it('should close the dialog', () => {
      const dialog = el.shadowRoot?.querySelector('dialog');
      expect(dialog?.open).to.be.false;
    });
  });

  describe('when open changes but there is no dialog element', () => {
    let el: TestNativeDialog;

    beforeEach(async () => {
      el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
      // Simulate no dialog in shadow root by removing it
      el.shadowRoot?.querySelector('dialog')?.remove();
      // Should not throw
      el.open = true;
      await el.updateComplete;
    });

    it('should not throw', () => {
      expect(true).to.be.true;
    });
  });

  describe('_handleDialogCancel', () => {
    let el: TestNativeDialog;
    let event: Event;
    let preventDefaultSpy: sinon.SinonSpy;

    beforeEach(async () => {
      el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
      el.open = true;
      await el.updateComplete;

      event = new Event('cancel', { cancelable: true });
      preventDefaultSpy = sinon.spy(event, 'preventDefault');
      el._handleDialogCancel(event);
      await el.updateComplete;
    });

    afterEach(() => {
      el.open = false;
    });

    it('should call preventDefault', () => {
      expect(preventDefaultSpy).to.have.been.calledOnce;
    });

    it('should close the dialog by calling _closeDialog', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('_handleDialogClick', () => {
    describe('when click target matches currentTarget (backdrop click)', () => {
      let el: TestNativeDialog;

      beforeEach(async () => {
        el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
        el.open = true;
        await el.updateComplete;

        const dialog = el.shadowRoot!.querySelector('dialog')!;
        const event = new MouseEvent('click', { bubbles: true, composed: true });
        Object.defineProperty(event, 'target', { value: dialog });
        Object.defineProperty(event, 'currentTarget', { value: dialog });
        el._handleDialogClick(event);
        await el.updateComplete;
      });

      it('should close the dialog', () => {
        expect(el.open).to.be.false;
      });
    });

    describe('when click target does not match currentTarget (content click)', () => {
      let el: TestNativeDialog;

      beforeEach(async () => {
        el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
        el.open = true;
        await el.updateComplete;

        const dialog = el.shadowRoot!.querySelector('dialog')!;
        const innerDiv = document.createElement('div');
        const event = new MouseEvent('click', { bubbles: true, composed: true });
        Object.defineProperty(event, 'target', { value: innerDiv });
        Object.defineProperty(event, 'currentTarget', { value: dialog });
        el._handleDialogClick(event);
        await el.updateComplete;
      });

      it('should not close the dialog', () => {
        expect(el.open).to.be.true;
      });
    });
  });

  describe('when disconnected from DOM', () => {
    let el: TestNativeDialog;

    beforeEach(async () => {
      const button = document.createElement('button');
      document.body.appendChild(button);
      button.focus();

      el = await fixture<TestNativeDialog>(html`<test-native-dialog></test-native-dialog>`);
      el.open = true;
      await el.updateComplete;

      button.remove();
      el.remove();
    });

    it('should clear _previouslyFocusedElement', () => {
      expect(el.previouslyFocusedElement).to.be.null;
    });
  });
});
