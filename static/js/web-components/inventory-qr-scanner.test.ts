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
    getFrontmatter: sinon.stub().resolves({
      frontmatter: {
        toJson: () => ({
          title: 'Test Page',
          inventory: {
            container: 'parent_container',
            is_container: true,
          },
        }),
      },
    }),
  };
}

/**
 * Helper to get the inner qr-scanner element
 */
function getInnerScanner(el: InventoryQrScanner): QrScanner | null {
  return el.querySelector('qr-scanner') as QrScanner | null;
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
  await (el as unknown as { _handleQrScanned: (e: CustomEvent<QrScannedEventDetail>) => Promise<void> })._handleQrScanned(event);
}

describe('InventoryQrScanner', () => {
  describe('when component is initialized', () => {
    let el: InventoryQrScanner;

    beforeEach(async () => {
      el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
    });

    it('should render scanner header', () => {
      const header = el.querySelector('.scanner-header');
      expect(header).to.exist;
    });

    it('should show "Scan QR Code" title', () => {
      const title = el.querySelector('.scanner-header .title');
      expect(title?.textContent).to.include('Scan QR Code');
    });

    it('should show Cancel button', () => {
      const cancelBtn = el.querySelector('.cancel-button');
      expect(cancelBtn).to.exist;
      expect(cancelBtn?.textContent).to.include('Cancel');
    });

    it('should contain embedded qr-scanner', () => {
      const scanner = getInnerScanner(el);
      expect(scanner).to.exist;
      expect(scanner?.getAttribute('embedded')).to.not.be.null;
    });

    it('should not show error container', () => {
      const errorContainer = el.querySelector('.error-container');
      expect(errorContainer).to.not.exist;
    });

    it('should not show validating spinner', () => {
      const validating = el.querySelector('.validating');
      expect(validating).to.not.exist;
    });
  });

  describe('Cancel button', () => {
    describe('when clicked', () => {
      let el: InventoryQrScanner;
      let cancelledSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);
        cancelledSpy = sinon.spy();
        el.addEventListener('cancelled', cancelledSpy);

        const cancelBtn = el.querySelector('.cancel-button') as HTMLButtonElement;
        cancelBtn?.click();
        await el.updateComplete;
      });

      it('should emit cancelled event', () => {
        expect(cancelledSpy).to.have.been.calledOnce;
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
        (el as unknown as { frontmatterClient: MockFrontmatterClient }).frontmatterClient = mockClient;

        // Stub collapse to avoid qr-scanner interaction
        const innerScanner = getInnerScanner(el)!;
        collapseStub = sinon.stub(innerScanner, 'collapse').resolves();

        // Set up event spy
        itemScannedSpy = sinon.spy();
        el.addEventListener('item-scanned', (e) => {
          scannedEvent = e as CustomEvent<ItemScannedEventDetail>;
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

      it('should show error container', () => {
        const errorContainer = el.querySelector('.error-container');
        expect(errorContainer).to.exist;
      });

      it('should show error message about invalid URL', () => {
        const errorMessage = el.querySelector('.error-message');
        expect(errorMessage?.textContent).to.include('Not a valid wiki URL');
      });

      it('should show Scan Again button', () => {
        const scanAgainBtn = el.querySelector('.scan-again-button');
        expect(scanAgainBtn).to.exist;
        expect(scanAgainBtn?.textContent).to.include('Scan Again');
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

      it('should show error container', () => {
        const errorContainer = el.querySelector('.error-container');
        expect(errorContainer).to.exist;
      });

      it('should show error message', () => {
        const errorMessage = el.querySelector('.error-message');
        expect(errorMessage?.textContent).to.include('Page not found');
      });
    });
  });

  describe('Scan Again button', () => {
    describe('when clicked after error', () => {
      let el: InventoryQrScanner;
      let expandSpy: SinonSpy;

      beforeEach(async () => {
        el = await fixture(html`<inventory-qr-scanner></inventory-qr-scanner>`);

        // Spy on expand method before triggering error
        expandSpy = sinon.spy(el, 'expand');

        // First trigger an error
        await simulateQrScan(el, 'invalid');
        await el.updateComplete;

        // Click Scan Again (which will clear error and show scanner again)
        const scanAgainBtn = el.querySelector('.scan-again-button') as HTMLButtonElement;
        scanAgainBtn?.click();
        await el.updateComplete;
      });

      afterEach(() => {
        expandSpy.restore();
      });

      it('should clear the error', () => {
        const errorContainer = el.querySelector('.error-container');
        expect(errorContainer).to.not.exist;
      });

      it('should call expand', () => {
        expect(expandSpy).to.have.been.calledOnce;
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
        const cancelBtn = el.querySelector('.cancel-button') as HTMLButtonElement;
        expect(cancelBtn?.disabled).to.be.true;
      });
    });
  });
});
