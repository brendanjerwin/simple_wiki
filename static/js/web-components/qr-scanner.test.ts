import { html, fixture, expect } from '@open-wc/testing';
import type { SinonStub, SinonSpy } from 'sinon';
import sinon from 'sinon';
import './qr-scanner.js';
import type { QrScanner, CameraProvider, CameraDevice, ScannerErrorEventDetail } from './qr-scanner.js';
import { AugmentedError } from './augment-error-service.js';
import type { ErrorDisplay } from './error-display.js';

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
      expect(el.expanded).to.equal(false);
    });

    it('should show the toggle button', () => {
      const button = el.shadowRoot?.querySelector('.scanner-toggle');
      expect(button).to.not.equal(null);
    });

    it('should have scanner area hidden', () => {
      const area = el.shadowRoot?.querySelector('.scanner-area');
      expect(area?.classList.contains('collapsed')).to.equal(true);
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
        expect(el.expanded).to.equal(true);
      });

      it('should request camera list', () => {
        expect(mockProvider.getCameras.called).to.equal(true);
      });

      it('should start scanning', () => {
        expect(mockProvider.start.called).to.equal(true);
      });

      it('should prefer back camera', () => {
        const startCall = mockProvider.start.firstCall;
        expect(startCall.args[1]).to.equal('camera1');
      });

      it('should show scanner area', () => {
        const area = el.shadowRoot?.querySelector('.scanner-area');
        expect(area?.classList.contains('collapsed')).to.equal(false);
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
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('No camera');
      });

      it('should not start scanning', () => {
        expect(mockProvider.start.called).to.equal(false);
      });
    });

    describe('when camera permission is denied', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.rejects(new DOMException('Permission denied', 'NotAllowedError'));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show permission error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('denied');
      });

      it('should emit scanner-error event', () => {
        expect(errorSpy.callCount).to.equal(1);
      });

      it('should not start scanning', () => {
        expect(mockProvider.start.called).to.equal(false);
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
        expect(el.expanded).to.equal(false);
      });

      it('should call stop on camera provider', () => {
        expect(mockProvider.stop.called).to.equal(true);
      });

      it('should hide scanner area', () => {
        const area = el.shadowRoot?.querySelector('.scanner-area');
        expect(area?.classList.contains('collapsed')).to.equal(true);
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
        expect(el.expanded).to.equal(true);
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
        expect(el.expanded).to.equal(false);
      });
    });
  });

  describe('scan success', () => {
    describe('when QR code is scanned via CameraProvider callback', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let scannedSpy: SinonSpy;
      let onSuccessCallback: (result: string) => void;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);
        // Capture the onSuccess callback passed to start()
        mockProvider.start.callsFake(
          async (_video: HTMLVideoElement, _cameraId: string, onSuccess: (result: string) => void) => {
            onSuccessCallback = onSuccess;
          }
        );

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;

        scannedSpy = sinon.spy();
        el.addEventListener('qr-scanned', scannedSpy);

        // Trigger scan success via the captured callback
        onSuccessCallback('https://wiki.example.com/toolbox/view');
        await el.updateComplete;
      });

      it('should dispatch qr-scanned event', () => {
        expect(scannedSpy.callCount).to.equal(1);
      });

      it('should include raw value in event detail', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = scannedSpy.firstCall.args[0] as CustomEvent;
        expect(event.detail.rawValue).to.equal('https://wiki.example.com/toolbox/view');
      });

      it('should auto-collapse after scan', () => {
        expect(el.expanded).to.equal(false);
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

        const stopButton = el.shadowRoot?.querySelector<HTMLButtonElement>('.stop-button');
        stopButton?.click();
        await el.updateComplete;
      });

      it('should collapse the scanner', () => {
        expect(el.expanded).to.equal(false);
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
        expect(select).to.not.equal(null);
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
        expect(select).to.equal(null);
      });
    });
  });

  describe('_handleError', () => {
    describe('when error is a string', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        // Use returns with Promise.reject to avoid Sinon wrapping the string in an Error
        mockProvider.getCameras.returns(Promise.reject('String error from library'));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show the string as error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('String error from library');
      });

      it('should emit scanner-error event', () => {
        expect(errorSpy.callCount).to.equal(1);
      });

      it('should wrap string in Error object', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error).to.be.instanceOf(Error);
        expect(event.detail.error.message).to.equal('String error from library');
      });
    });

    describe('when error is null', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        // Use returns with Promise.reject to avoid Sinon wrapping
        mockProvider.getCameras.returns(Promise.reject(null));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show unknown error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('Unknown error (null or undefined)');
      });

      it('should emit scanner-error event with Error object', () => {
        expect(errorSpy.callCount).to.equal(1);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error).to.be.instanceOf(Error);
      });
    });

    describe('when error is undefined', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        // Use returns with Promise.reject to avoid Sinon wrapping
        mockProvider.getCameras.returns(Promise.reject(undefined));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show unknown error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('Unknown error (null or undefined)');
      });

      it('should emit scanner-error event with Error object', () => {
        expect(errorSpy.callCount).to.equal(1);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error).to.be.instanceOf(Error);
      });
    });

    describe('when error is an object (not Error)', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        // Use returns with Promise.reject to avoid Sinon wrapping
        mockProvider.getCameras.returns(Promise.reject({ code: 123, reason: 'custom object' }));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show unknown error message with object type', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('Unknown error:');
        expect(errorDisplay?.augmentedError?.message).to.include('[object Object]');
      });

      it('should emit scanner-error event with Error object', () => {
        expect(errorSpy.callCount).to.equal(1);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error).to.be.instanceOf(Error);
      });

      it('should preserve original error as cause', () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error.cause).to.deep.equal({ code: 123, reason: 'custom object' });
      });
    });

    describe('when error is a primitive (number)', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        // Use returns with Promise.reject to avoid Sinon wrapping
        mockProvider.getCameras.returns(Promise.reject(42));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show unknown error message with stringified value', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('Unknown error: 42');
      });

      it('should emit scanner-error event with Error object', () => {
        expect(errorSpy.callCount).to.equal(1);
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing CustomEvent from spy
        const event = errorSpy.firstCall.args[0] as CustomEvent<ScannerErrorEventDetail>;
        expect(event.detail.error).to.be.instanceOf(Error);
      });
    });

    describe('when error is PermissionDeniedError DOMException', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;
      let errorSpy: SinonSpy;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.rejects(new DOMException('Permission denied', 'PermissionDeniedError'));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        errorSpy = sinon.spy();
        el.addEventListener('scanner-error', errorSpy);

        await el.expand();
        await el.updateComplete;
      });

      it('should show camera permission error message', () => {
        const errorDisplay = el.shadowRoot?.querySelector<ErrorDisplay>('error-display');
        expect(errorDisplay).to.not.equal(null);
        expect(errorDisplay?.augmentedError).to.be.instanceOf(AugmentedError);
        expect(errorDisplay?.augmentedError?.message).to.include('denied');
      });

      it('should emit scanner-error event', () => {
        expect(errorSpy.callCount).to.equal(1);
      });
    });
  });

  describe('_handleCameraChange type guard', () => {
    describe('when camera change event target is not HTMLSelectElement', () => {
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

        // Reset the start stub call count to detect if it's called again
        mockProvider.start.resetHistory();

        // Dispatch change event with non-select target (e.g., div element)
        const fakeEvent = new Event('change', { bubbles: true });
        Object.defineProperty(fakeEvent, 'target', { value: document.createElement('div') });
        const select = el.shadowRoot?.querySelector('#camera-select');
        select?.dispatchEvent(fakeEvent);

        // Wait a tick for any async handlers to potentially fire (they shouldn't)
        await new Promise(r => setTimeout(r, 0));
        await el.updateComplete;
      });

      it('should not restart scanning', () => {
        expect(mockProvider.start.called).to.equal(false);
      });
    });
  });

  describe('embedded mode', () => {
    describe('when embedded attribute is set', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);

        el = await fixture(html`<qr-scanner embedded></qr-scanner>`);
        el.setCameraProvider(mockProvider);
        await el.updateComplete;
      });

      it('should not show toggle button', () => {
        const button = el.shadowRoot?.querySelector('.scanner-toggle');
        expect(button).to.equal(null);
      });

      it('should show scanner area by default', () => {
        const area = el.shadowRoot?.querySelector('.scanner-area');
        expect(area?.classList.contains('collapsed')).to.equal(false);
      });
    });

    describe('when embedded and expanded', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([
          { id: 'camera1', label: 'Front Camera' },
          { id: 'camera2', label: 'Back Camera' },
        ]);

        el = await fixture(html`<qr-scanner embedded></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should not show camera select dropdown', () => {
        const select = el.shadowRoot?.querySelector('#camera-select');
        expect(select).to.equal(null);
      });

      it('should not show stop button', () => {
        const stopButton = el.shadowRoot?.querySelector('.stop-button');
        expect(stopButton).to.equal(null);
      });
    });
  });

  describe('_stopScanning', () => {
    describe('when stop throws an error', () => {
      let el: QrScanner;
      let mockProvider: ReturnType<typeof createMockCameraProvider>;

      beforeEach(async () => {
        mockProvider = createMockCameraProvider();
        mockProvider.getCameras.resolves([{ id: 'cam1', label: 'Camera 1' }]);
        mockProvider.isScanning.returns(true);
        mockProvider.stop.rejects(new Error('Stop failed'));

        el = await fixture(html`<qr-scanner></qr-scanner>`);
        el.setCameraProvider(mockProvider);

        await el.expand();
        await el.updateComplete;
      });

      it('should silently handle the error and still collapse', async () => {
        // This should not throw
        await el.collapse();
        await el.updateComplete;

        expect(el.expanded).to.equal(false);
      });
    });
  });
});
