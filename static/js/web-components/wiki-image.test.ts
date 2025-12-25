import { html, fixture, expect } from '@open-wc/testing';
import './wiki-image.js';
import type { WikiImage } from './wiki-image.js';
import sinon, { SinonStub } from 'sinon';

describe('WikiImage', () => {
  let el: WikiImage;

  it('should exist', async () => {
    el = await fixture(html`<wiki-image></wiki-image>`);
    expect(el).to.exist;
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
          title="Click to view"
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

    it('should have tool buttons (2 without HTTPS, 3 with HTTPS)', () => {
      const buttons = el.shadowRoot?.querySelectorAll('.tool-btn');
      // Copy button only appears in secure context with clipboard.write support
      const expectedCount = window.isSecureContext && navigator.clipboard && 'write' in navigator.clipboard ? 3 : 2;
      expect(buttons?.length).to.equal(expectedCount);
    });

    it('should have open in new tab button with aria-label', () => {
      const openBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Open in new tab"]');
      expect(openBtn).to.exist;
    });

    it('should have download button with aria-label', () => {
      const downloadBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Download"]');
      expect(downloadBtn).to.exist;
    });

    it('should have copy image button with aria-label (only in secure context)', () => {
      const copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]');
      if (window.isSecureContext && navigator.clipboard && 'write' in navigator.clipboard) {
        expect(copyBtn).to.exist;
      } else {
        expect(copyBtn).to.not.exist;
      }
    });

    it('should have toolbar role for accessibility', () => {
      const toolsPanel = el.shadowRoot?.querySelector('.tools-panel');
      expect(toolsPanel?.getAttribute('role')).to.equal('toolbar');
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
      const openBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Open in new tab"]') as HTMLButtonElement;
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
          createdLink = element as HTMLAnchorElement;
          sinon.stub(createdLink, 'click');
        }
        return element;
      });

      el = await fixture(html`
        <wiki-image src="/uploads/test-image.jpg" alt="Test"></wiki-image>
      `);
      const downloadBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Download"]') as HTMLButtonElement;
      downloadBtn?.click();
    });

    afterEach(() => {
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
        const copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]') as HTMLButtonElement;
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

    describe('when not in secure context', () => {
      let originalIsSecureContext: boolean;
      let toastElement: Element | null;

      beforeEach(async () => {
        originalIsSecureContext = window.isSecureContext;
        Object.defineProperty(window, 'isSecureContext', { value: false, writable: true });

        el = await fixture(html`
          <wiki-image src="/uploads/test.jpg" alt="Test"></wiki-image>
        `);
        const copyBtn = el.shadowRoot?.querySelector('.tool-btn[aria-label="Copy image"]') as HTMLButtonElement;
        copyBtn?.click();

        await new Promise(resolve => setTimeout(resolve, 50));
        toastElement = document.querySelector('toast-message');
      });

      afterEach(() => {
        Object.defineProperty(window, 'isSecureContext', { value: originalIsSecureContext, writable: true });
        document.querySelectorAll('toast-message').forEach(t => t.remove());
      });

      it('should show error toast', () => {
        expect(toastElement).to.exist;
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
        const img = el.shadowRoot?.querySelector('img') as HTMLImageElement;
        img?.click();
      });

      it('should open toolsOpen', () => {
        expect(el.toolsOpen).to.be.true;
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
        const closeBtn = el.shadowRoot?.querySelector('.close-btn') as HTMLButtonElement;
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
    let removeEventListenerSpy: SinonStub;

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
        expect((el as unknown as { _getFilename: () => string })._getFilename()).to.equal('image');
      });
    });
  });
});
