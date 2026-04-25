import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { handleKeydownFocusTrap } from './native-dialog-mixin.js';

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
