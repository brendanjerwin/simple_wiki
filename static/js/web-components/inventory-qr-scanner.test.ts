import { html, fixture, expect } from '@open-wc/testing';
import sinon, { SinonStub, SinonSpy } from 'sinon';
import './inventory-qr-scanner.js';
import { InventoryQrScanner, ItemScannedEventDetail } from './inventory-qr-scanner.js';
import type { QrScanner, QrScannedEventDetail } from './qr-scanner.js';

/**
 * Mock Frontmatter client for testing
 */
interface MockFrontmatterClient {
  getFrontmatter: SinonStub;
}

function createMockFrontmatterClient(): MockFrontmatterClient {
  return {
    // In protobuf-es v2, frontmatter is already a JsonObject (no toJson() method)
    getFrontmatter: sinon.stub().resolves({
      frontmatter: {
        title: 'Test Page',
        inventory: {
          container: 'parent_container',
          is_container: true,
        },
      },
    }),
  };
}

/**
 * Helper to get the inner qr-scanner element
 */
function getInnerScanner(el: InventoryQrScanner): QrScanner | null {
  return el.shadowRoot?.querySelector<QrScanner>('qr-scanner') ?? null;
}

/**
 * Helper to simulate a QR scan on the component.
 * Uses type assertion to access private handler for testing.
 */
async function simulateQrScan(el: InventoryQrScanner, rawValue: string): Promise<void> {
  const event = new CustomEvent<QrScannedEventDetail>('qr-scanned', {
    detail: { rawValue },
    bubbles: true,
    composed: true,
  });
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
  const handler = (el as unknown as { _handleQrScanned: (e: CustomEvent<QrScannedEventDetail>) => Promise<void> });
  await handler._handleQrScanned(event);
}

describe('InventoryQrScanner', () => {
  describe('when component is initialized', () => {
    let el: InventoryQrScanner;

    beforeEach(async () => {
      el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
    });

    it('should render scanner header', () => {
      const header = el.shadowRoot?.querySelector('.scanner-header');
      expect(header).to.exist;
    });

    it('should show "Scan QR Code" title', () => {
      const title = el.shadowRoot?.querySelector('.scanner-header .title');
      expect(title?.textContent).to.include('Scan QR Code');
    });

    it('should show Cancel button', () => {
      const cancelBtn = el.shadowRoot?.querySelector('.cancel-button');
      expect(cancelBtn).to.exist;
      expect(cancelBtn?.textContent).to.include('Cancel');
    });

    it('should contain embedded qr-scanner', () => {
      const scanner = getInnerScanner(el);
      expect(scanner).to.exist;
      expect(scanner?.getAttribute('embedded')).to.not.be.null;
    });

    it('should not show error-display', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.not.exist;
    });

    it('should not show validating spinner', () => {
      const validating = el.shadowRoot?.querySelector('.validating');
      expect(validating).to.not.exist;
    });
  });

  describe('Cancel button', () => {
    describe('when clicked', () => {
      let el: InventoryQrScanner;
      let cancelledSpy: SinonSpy;
      let collapseStub: SinonStub;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
        // Stub collapse to prevent camera access that crashes headless browser.
        // Trade-off: We verify 'collapse' is called, but not its actual behavior.
        // The collapse method's behavior is tested separately in qr-scanner.test.ts.
        collapseStub = sinon.stub(el, 'collapse').resolves();
        cancelledSpy = sinon.spy();
        el.addEventListener('cancelled', cancelledSpy);

        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.cancel-button');
        cancelBtn?.click();
        await el.updateComplete;
      });

      afterEach(() => {
        collapseStub.restore();
      });

      it('should emit cancelled event', () => {
        expect(cancelledSpy).to.have.been.calledOnce;
      });

      it('should call collapse', () => {
        expect(collapseStub).to.have.been.calledOnce;
      });
    });
  });

  describe('expand', () => {
    describe('when called', () => {
      let el: InventoryQrScanner;
      let innerScanner: QrScanner;
      let expandStub: SinonStub;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
        innerScanner = getInnerScanner(el)!;
        expandStub = sinon.stub(innerScanner, 'expand').resolves();

        await el.expand();
      });

      afterEach(() => {
        expandStub.restore();
      });

      it('should call expand on inner qr-scanner', () => {
        expect(expandStub).to.have.been.calledOnce;
      });
    });
  });

  describe('collapse', () => {
    describe('when called', () => {
      let el: InventoryQrScanner;
      let innerScanner: QrScanner;
      let collapseStub: SinonStub;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
        innerScanner = getInnerScanner(el)!;
        collapseStub = sinon.stub(innerScanner, 'collapse').resolves();

        await el.collapse();
      });

      afterEach(() => {
        collapseStub.restore();
      });

      it('should call collapse on inner qr-scanner', () => {
        expect(collapseStub).to.have.been.calledOnce;
      });
    });
  });

  describe('_handleQrScanned', () => {
    describe('when valid wiki URL is scanned', () => {
      let el: InventoryQrScanner;
      let itemScannedSpy: SinonSpy;
      let mockClient: MockFrontmatterClient;
      let collapseStub: SinonStub;
      let scannedEvent: CustomEvent<ItemScannedEventDetail>;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);

        // Mock the frontmatter client
        mockClient = createMockFrontmatterClient();
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as { frontmatterClient: MockFrontmatterClient }).frontmatterClient = mockClient;

        // Stub collapse to avoid qr-scanner interaction
        const innerScanner = getInnerScanner(el)!;
        collapseStub = sinon.stub(innerScanner, 'collapse').resolves();

        // Set up event spy
        itemScannedSpy = sinon.spy();
        el.addEventListener('item-scanned', (e) => {
          if (e instanceof CustomEvent) {
            scannedEvent = e;
          }
          itemScannedSpy(e);
        });

        // Simulate scan and wait for async processing
        await simulateQrScan(el, 'https://wiki.example.com/test_page/view');
        await el.updateComplete;
      });

      afterEach(() => {
        collapseStub.restore();
      });

      it('should call frontmatter service', () => {
        expect(mockClient.getFrontmatter).to.have.been.calledOnce;
      });

      it('should emit item-scanned event', () => {
        expect(itemScannedSpy).to.have.been.calledOnce;
      });

      it('should include identifier in event detail', () => {
        expect(scannedEvent.detail.item.identifier).to.equal('test_page');
      });

      it('should include title in event detail', () => {
        expect(scannedEvent.detail.item.title).to.equal('Test Page');
      });

      it('should include container in event detail', () => {
        expect(scannedEvent.detail.item.container).to.equal('parent_container');
      });

      it('should include isContainer in event detail', () => {
        expect(scannedEvent.detail.item.isContainer).to.be.true;
      });

      it('should collapse the scanner', () => {
        expect(collapseStub).to.have.been.called;
      });
    });

    describe('when invalid URL is scanned', () => {
      let el: InventoryQrScanner;
      let itemScannedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);

        itemScannedSpy = sinon.spy();
        el.addEventListener('item-scanned', itemScannedSpy);

        // Simulate scan with invalid URL
        await simulateQrScan(el, 'not a valid url with no path');
        await el.updateComplete;
      });

      it('should not emit item-scanned event', () => {
        expect(itemScannedSpy).to.not.have.been.called;
      });

      it('should show error-display', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should show Scan Again button via error-display action', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
        // The action button is rendered by error-display with the label we provided
        const actionBtn = errorDisplay?.shadowRoot?.querySelector<HTMLButtonElement>('.action-button');
        expect(actionBtn).to.exist;
        expect(actionBtn?.textContent).to.include('Scan Again');
      });
    });

    describe('when page is not found', () => {
      let el: InventoryQrScanner;
      let mockClient: MockFrontmatterClient;
      let itemScannedSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);

        // Mock the frontmatter client to throw error
        mockClient = createMockFrontmatterClient();
        mockClient.getFrontmatter.rejects(new Error('Page not found'));
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
        (el as unknown as { frontmatterClient: MockFrontmatterClient }).frontmatterClient = mockClient;

        itemScannedSpy = sinon.spy();
        el.addEventListener('item-scanned', itemScannedSpy);

        // Simulate scan and wait for async processing
        await simulateQrScan(el, 'https://wiki.example.com/nonexistent_page/view');
        await el.updateComplete;
      });

      it('should not emit item-scanned event', () => {
        expect(itemScannedSpy).to.not.have.been.called;
      });

      it('should show error-display', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });
    });
  });

  describe('Scan Again button', () => {
    describe('when clicked after error', () => {
      let el: InventoryQrScanner;
      let expandStub: SinonStub;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);

        // Stub expand method to prevent real camera access in headless browser.
        // Trade-off: We verify 'expand' is called, but not its actual behavior.
        // The expand method's behavior is tested separately in qr-scanner.test.ts.
        expandStub = sinon.stub(el, 'expand').resolves();

        // First trigger an error
        await simulateQrScan(el, 'invalid');
        await el.updateComplete;

        // Click Scan Again button inside error-display
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        const scanAgainBtn = errorDisplay?.shadowRoot?.querySelector<HTMLButtonElement>('.action-button');
        scanAgainBtn?.click();
        await el.updateComplete;
      });

      afterEach(() => {
        expandStub.restore();
      });

      it('should clear the error', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.not.exist;
      });

      it('should call expand', () => {
        expect(expandStub).to.have.been.calledOnce;
      });
    });
  });

  describe('disabled property', () => {
    describe('when disabled is true', () => {
      let el: InventoryQrScanner;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner disabled></inventory-qr-scanner>`);
      });

      it('should disable Cancel button', () => {
        const cancelBtn = el.shadowRoot?.querySelector<HTMLButtonElement>('.cancel-button');
        expect(cancelBtn?.disabled).to.be.true;
      });
    });
  });
});
