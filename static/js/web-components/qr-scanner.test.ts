import { html, fixture, expect } from '@open-wc/testing';
import sinon, { SinonStub, SinonSpy } from 'sinon';
import './qr-scanner.js';
import { QrScanner, CameraProvider, CameraDevice } from './qr-scanner.js';

/**
 * Mock camera provider for testing
 */
function createMockCameraProvider(): CameraProvider & {
  getCameras: SinonStub;
  start: SinonStub;
  stop: SinonStub;
  isScanning: SinonStub;
} {
  return {
    getCameras: sinon.stub().resolves([]),
    start: sinon.stub().resolves(),
    stop: sinon.stub().resolves(),
    isScanning: sinon.stub().returns(false),
  };
}

describe('QrScanner', () => {
  describe('when component is initialized', () => {
    let el: QrScanner;

    beforeEach(async () => {
      el = await fixture(html`<qr-scanner></qr-scanner>`);
    });

    it('should be collapsed by default', () => {
      expect(el.expanded).to.be.false;
    });

    it('should show the toggle button', () => {
      const button = el.shadowRoot?.querySelector('.scanner-toggle');
      expect(button).to.exist;
    });

    it('should have scanner area hidden', () => {
      const area = el.shadowRoot?.querySelector('.scanner-area');
      expect(area?.classList.contains('collapsed')).to.be.true;
    });

    it('should show "Scan QR Code" text', () => {
      const button = el.shadowRoot?.querySelector('.scanner-toggle');
      expect(button?.textContent).to.include('Scan QR Code');
    });
  });

  describe('expand', () => {
    describe('when cameras are available', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      const mockCameras: CameraDevice[] = [
        { id: 'camera1', label: 'Back Camera' },
        { id: 'camera2', label: 'Front Camera' },
      ];

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves(mockCameras);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should set expanded to true', () => {
        expect(el.expanded).to.be.true;
      });

      it('should request camera list', () => {
        expect(mockProvider.getCameras).to.have.been.called;
      });

      it('should start scanning', () => {
        expect(mockProvider.start).to.have.been.called;
      });

      it('should prefer back camera', () => {
        const startCall = mockProvider.start.firstCall;
        expect(startCall.args[1]).to.equal('camera1');
      });

      it('should show scanner area', () => {
        const area = el.shadowRoot?.querySelector('.scanner-area');
        expect(area?.classList.contains('collapsed')).to.be.false;
      });

      it('should show "Close Scanner" text', () => {
        const button = el.shadowRoot?.querySelector('.scanner-toggle');
        expect(button?.textContent).to.include('Close Scanner');
      });
    });

    describe('when no cameras are available', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should show error message', () => {
        const error = el.shadowRoot?.querySelector('.error-message');
        expect(error).to.exist;
        expect(error?.textContent).to.include('No camera');
      });

      it('should not start scanning', () => {
        expect(mockProvider.start).to.not.have.been.called;
      });
    });

    describe('when camera permission is denied', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.rejects(new Error('NotAllowedError: Permission denied'));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show permission error message', () => {
        const error = el.shadowRoot?.querySelector('.error-message');
        expect(error).to.exist;
        expect(error?.textContent).to.include('denied');
      });

      it('should emit scanner-error event', () => {
        expect(errorSpy).to.have.been.calledOnce;
      });

      it('should not start scanning', () => {
        expect(mockProvider.start).to.not.have.been.called;
      });
    });
  });

  describe('collapse', () => {
    describe('when scanner is expanded and scanning', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);
        mockProvider.isScanning.returns(true);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;

        await el.collapse();
        await el.updateComplete;
      });

      it('should set expanded to false', () => {
        expect(el.expanded).to.be.false;
      });

      it('should call stop on camera provider', () => {
        expect(mockProvider.stop).to.have.been.called;
      });

      it('should hide scanner area', () => {
        const area = el.shadowRoot?.querySelector('.scanner-area');
        expect(area?.classList.contains('collapsed')).to.be.true;
      });
    });
  });

  describe('toggle', () => {
    describe('when collapsed', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.toggle();
        await el.updateComplete;
      });

      it('should expand', () => {
        expect(el.expanded).to.be.true;
      });
    });

    describe('when expanded', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;

        await el.toggle();
        await el.updateComplete;
      });

      it('should collapse', () => {
        expect(el.expanded).to.be.false;
      });
    });
  });

  describe('_onScanSuccess', () => {
    describe('when QR code is scanned', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let scannedSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;

        scannedSpy = sinon.spy();
        el.addEventListener('qr-scanned', scannedSpy);

        el._onScanSuccess('https://wiki.example.com/toolbox/view');
        await el.updateComplete;
      });

      it('should dispatch qr-scanned event', () => {
        expect(scannedSpy).to.have.been.calledOnce;
      });

      it('should include raw value in event detail', () => {
        const event = scannedSpy.firstCall.args[0] as CustomEvent;
        expect(event.detail.rawValue).to.equal('https://wiki.example.com/toolbox/view');
      });

      it('should auto-collapse after scan', () => {
        expect(el.expanded).to.be.false;
      });
    });
  });

  describe('stop button', () => {
    describe('when clicked', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;

        const stopButton = el.shadowRoot?.querySelector('.stop-button') as HTMLButtonElement;
        stopButton?.click();
        await el.updateComplete;
      });

      it('should collapse the scanner', () => {
        expect(el.expanded).to.be.false;
      });
    });
  });

  describe('camera selection', () => {
    describe('when multiple cameras are available', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      const mockCameras: CameraDevice[] = [
        { id: 'camera1', label: 'Front Camera' },
        { id: 'camera2', label: 'Back Camera' },
      ];

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves(mockCameras);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should show camera select dropdown', () => {
        const select = el.shadowRoot?.querySelector('#camera-select');
        expect(select).to.exist;
      });

      it('should have options for each camera', () => {
        const options = el.shadowRoot?.querySelectorAll('#camera-select option');
        expect(options?.length).to.equal(2);
      });
    });

    describe('when only one camera is available', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should not show camera select dropdown', () => {
        const select = el.shadowRoot?.querySelector('#camera-select');
        expect(select).to.not.exist;
      });
    });
  });
});
