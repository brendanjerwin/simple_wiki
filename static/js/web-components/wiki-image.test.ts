import { html, fixture, expect } from '@open-wc/testing';
import './wiki-image.js';
import type { WikiImage } from './wiki-image.js';
import sinon, { SinonStub, SinonSpy } from 'sinon';

describe('WikiImage', () => {
  let el: WikiImage;

  describe('when created', () => {
    beforeEach(async () => {
      el = await fixture(html`<wiki-image></wiki-image>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });
  });

  describe('when rendering with src and alt', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test image"></wiki-image>
      `);
    });

    it('should render an img element', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img).to.exist;
    });

    it('should set the src attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.getAttribute('src')).to.equal('/uploads/test.jpg');
    });

    it('should set the alt attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.alt).to.equal('Test image');
    });
  });

  describe('when rendering with title', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image
          src="/uploads/test.jpg"
          alt="Test image"
          image-title="Click to view"
        ></wiki-image>
      `);
    });

    it('should set the title attribute', () => {
      const img = el.shadowRoot?.querySelector('img');
      expect(img?.title).to.equal('Click to view');
    });
  });

  describe('tools panel', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
    });

    it('should render a tools panel', () => {
      const toolsPanel = el.shadowRoot?.querySelector('.tools-panel');
      expect(toolsPanel).to.exist;
    });

    it('should have at least 2 tool buttons', () => {
      const buttons = el.shadowRoot?.querySelectorAll('.tool-btn');
      expect(buttons?.length).to.be.at.least(2);
    });

    it('should have open in new tab button with aria-label', () => {
      const openBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Open in new tab"]');
      expect(openBtn).to.exist;
    });

    it('should have download button with aria-label', () => {
      const downloadBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Download"]');
      expect(downloadBtn).to.exist;
    });

    it('should have toolbar role for accessibility', () => {
      const toolsPanel = el.shadowRoot?.querySelector('.tools-panel');
      expect(toolsPanel?.getAttribute('role')).to.equal('toolbar');
    });
  });

  describe('copy button visibility', () => {
    let originalIsSecureContext: PropertyDescriptor | undefined;
    let originalClipboard: PropertyDescriptor | undefined;

    beforeEach(() => {
      originalIsSecureContext = Object.getOwnPropertyDescriptor(window, 'isSecureContext');
      originalClipboard = Object.getOwnPropertyDescriptor(navigator, 'clipboard');
    });

    afterEach(() => {
      if (originalIsSecureContext) {
        Object.defineProperty(window, 'isSecureContext', originalIsSecureContext);
      }
      if (originalClipboard) {
        Object.defineProperty(navigator, 'clipboard', originalClipboard);
      }
    });

    describe('when in secure context with clipboard.write support', () => {
      let copyBtn: Element | null | undefined;

      beforeEach(async () => {
        Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true });
        Object.defineProperty(navigator, 'clipboard', {
          value: { write: () => Promise.resolve() },
          configurable: true
        });

        el = await fixture(html`<wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>`);
        copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]');
      });

      it('should render the copy button', () => {
        expect(copyBtn).to.exist;
      });
    });

    describe('when not in secure context', () => {
      let copyBtn: Element | null | undefined;

      beforeEach(async () => {
        Object.defineProperty(window, 'isSecureContext', { value: false, configurable: true });

        el = await fixture(html`<wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>`);
        copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]');
      });

      it('should not render the copy button', () => {
        expect(copyBtn).to.not.exist;
      });
    });

    describe('when clipboard API is not available', () => {
      let copyBtn: Element | null | undefined;

      beforeEach(async () => {
        Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true });
        Object.defineProperty(navigator, 'clipboard', {
          value: undefined,
          configurable: true
        });

        el = await fixture(html`<wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>`);
        copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]');
      });

      it('should not render the copy button', () => {
        expect(copyBtn).to.not.exist;
      });
    });
  });

  describe('toolsOpen property', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
    });

    it('should be false by default', () => {
      expect(el.toolsOpen).to.be.false;
    });

    describe('when set to true', () => {
      beforeEach(async () => {
        el.toolsOpen = true;
        await el.updateComplete;
      });

      it('should reflect to attribute', () => {
        expect(el.hasAttribute('tools-open')).to.be.true;
      });
    });
  });

  describe('when clicking open in new tab button', () => {
    let windowOpenStub: SinonStub;

    beforeEach(async () => {
      windowOpenStub = sinon.stub(window, 'open');
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
      const openBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.tool-btn[aria-label="Open in new tab"]');
      openBtn?.click();
    });

    afterEach(() => {
      windowOpenStub.restore();
    });

    it('should open the image in a new tab', () => {
      expect(windowOpenStub).to.have.been.calledWith('/uploads/test.jpg', '_blank');
    });
  });

  describe('when clicking download button', () => {
    let originalCreateElement: typeof document.createElement;
    let createdLink: HTMLAnchorElement | null = null;

    beforeEach(async () => {
      originalCreateElement = document.createElement.bind(document);

      sinon.stub(document, 'createElement').callsFake((tagName: string) => {
        const element = originalCreateElement(tagName);
        if (tagName === 'a') {
          // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- element is HTMLAnchorElement when tagName is 'a'
          createdLink = element as HTMLAnchorElement;
          sinon.stub(createdLink, 'click');
        }
        return element;
      });

      el = await fixture(html`
        <wiki-image src="/uploads/test-image.jpg" alt="Test"></wiki-image>
      `);
      const downloadBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.tool-btn[aria-label="Download"]');
      downloadBtn?.click();
    });

    afterEach(() => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- restoring sinon stub
      (document.createElement as SinonStub).restore();
    });

    it('should trigger a download', () => {
      expect(createdLink).to.exist;
      expect(createdLink?.click).to.have.been.calledOnce;
    });

    it('should set the download filename', () => {
      expect(createdLink?.download).to.equal('test-image.jpg');
    });

    it('should set the href to the image src', () => {
      expect(createdLink?.href).to.contain('/uploads/test-image.jpg');
    });
  });

  describe('when clicking copy image button', () => {
    describe('when clipboard write succeeds', () => {
      let clipboardWriteStub: SinonStub;
      let fetchStub: SinonStub;

      beforeEach(async () => {
        fetchStub = sinon.stub(window, 'fetch');
        fetchStub.resolves(new Response(new Blob(['test'], { type: 'image/jpeg' }), { status: 200 }));

        // Stub the clipboard.write method directly
        clipboardWriteStub = sinon.stub(navigator.clipboard, 'write');
        clipboardWriteStub.resolves();

        el = await fixture(html`
          <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
        `);
        const copyBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.tool-btn[aria-label="Copy image"]');
        copyBtn?.click();

        // Wait for async operation
        await new Promise(resolve => setTimeout(resolve, 50));
      });

      afterEach(() => {
        fetchStub.restore();
        clipboardWriteStub.restore();
      });

      it('should fetch the image', () => {
        expect(fetchStub).to.have.been.calledWith('/uploads/test.jpg');
      });

      it('should write to clipboard', () => {
        expect(clipboardWriteStub).to.have.been.calledOnce;
      });
    });
  });

  describe('click to open behavior', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
    });

    describe('when clicking the component', () => {
      beforeEach(() => {
        el.click();
      });

      it('should open toolsOpen', () => {
        expect(el.toolsOpen).to.be.true;
      });
    });

    describe('when clicking the component twice', () => {
      beforeEach(() => {
        el.click();
        el.click();
      });

      it('should keep toolsOpen true (no toggle)', () => {
        expect(el.toolsOpen).to.be.true;
      });
    });

    describe('when clicking the image inside shadow DOM', () => {
      beforeEach(() => {
        const img = el.shadowRoot?.querySelector<HTMLImageElement>('img');
        img?.click();
      });

      it('should open toolsOpen', () => {
        expect(el.toolsOpen).to.be.true;
      });
    });
  });

  describe('keyboard accessibility', () => {
    let img: HTMLImageElement | null | undefined;

    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test image"></wiki-image>
      `);
      img = el.shadowRoot?.querySelector('img');
    });

    it('should have tabindex on the image', () => {
      expect(img?.getAttribute('tabindex')).to.equal('0');
    });

    it('should have role button on the image', () => {
      expect(img?.getAttribute('role')).to.equal('button');
    });

    it('should have aria-label on the image', () => {
      expect(img?.getAttribute('aria-label')).to.equal('Test image - Press Enter to open tools');
    });

    describe('when pressing Enter on the image', () => {
      beforeEach(() => {
        const event = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true });
        img?.dispatchEvent(event);
      });

      it('should open the tools panel', () => {
        expect(el.toolsOpen).to.be.true;
      });
    });

    describe('when pressing Space on the image', () => {
      beforeEach(() => {
        const event = new KeyboardEvent('keydown', { key: ' ', bubbles: true });
        img?.dispatchEvent(event);
      });

      it('should open the tools panel', () => {
        expect(el.toolsOpen).to.be.true;
      });
    });

    describe('when pressing other keys on the image', () => {
      beforeEach(() => {
        const event = new KeyboardEvent('keydown', { key: 'a', bubbles: true });
        img?.dispatchEvent(event);
      });

      it('should not open the tools panel', () => {
        expect(el.toolsOpen).to.be.false;
      });
    });
  });

  describe('close button (mobile)', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
      el.toolsOpen = true;
      await el.updateComplete;
    });

    it('should render a close bar', () => {
      const closeBar = el.shadowRoot?.querySelector('.close-bar');
      expect(closeBar).to.exist;
    });

    it('should have close button with aria-label', () => {
      const closeBtn = el.shadowRoot?.querySelector('.close-btn[aria-label="Close tools"]');
      expect(closeBtn).to.exist;
    });

    describe('when clicking close button', () => {
      beforeEach(() => {
        const closeBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.close-btn');
        closeBtn?.click();
      });

      it('should close the tools panel', () => {
        expect(el.toolsOpen).to.be.false;
      });
    });
  });

  describe('click outside to close', () => {
    beforeEach(async () => {
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
      el.toolsOpen = true;
      await el.updateComplete;
    });

    describe('when clicking outside the component', () => {
      beforeEach(() => {
        document.body.click();
      });

      it('should close the tools panel', () => {
        expect(el.toolsOpen).to.be.false;
      });
    });
  });

  describe('event listener cleanup', () => {
    let removeEventListenerSpy: SinonSpy;

    beforeEach(async () => {
      removeEventListenerSpy = sinon.spy(document, 'removeEventListener');
      el = await fixture(html`
        <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
      `);
    });

    afterEach(() => {
      removeEventListenerSpy.restore();
    });

    describe('when component is disconnected', () => {
      beforeEach(() => {
        el.remove();
      });

      it('should remove click event listener', () => {
        expect(removeEventListenerSpy).to.have.been.calledWith('click');
      });
    });
  });

  describe('_getFilename', () => {
    describe('when src has a filename', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <wiki-image src="/uploads/my-image.jpg" alt="Test"></wiki-image>
        `);
      });

      it('should extract the filename', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        expect((el as unknown as { _getFilename: () => string })._getFilename()).to.equal('my-image.jpg');
      });
    });

    describe('when src has no filename', () => {
      beforeEach(async () => {
        el = await fixture(html`
          <wiki-image src="/" alt="Test"></wiki-image>
        `);
      });

      it('should return default filename', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
        expect((el as unknown as { _getFilename: () => string })._getFilename()).to.equal('image');
      });
    });
  });
});
