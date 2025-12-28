import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { sharedStyles, foundationCSS, buttonCSS, responsiveCSS } from './shared-styles.js';
import QrScannerLib from 'qr-scanner';

/**
 * Camera device information
 */
export interface CameraDevice {
  id: string;
  label: string;
}

/**
 * Interface for camera access - enables dependency injection for testing
 */
export interface CameraProvider {
  getCameras(): Promise<CameraDevice[]>;
  start(videoElement: HTMLVideoElement, cameraId: string, onSuccess: (result: string) => void): Promise<void>;
  stop(): Promise<void>;
  isScanning(): boolean;
}

/**
 * Event detail for qr-scanned event
 */
export interface QrScannedEventDetail {
  rawValue: string;
}

/**
 * Event detail for scanner-error event
 */
export interface ScannerErrorEventDetail {
  error: Error;
}

/**
 * Custom error types for camera operations
 */
export class CameraPermissionError extends Error {
  constructor(cause?: Error) {
    super('Camera access denied. Check browser settings.');
    this.name = 'CameraPermissionError';
    this.cause = cause;
  }
}

export class NoCameraError extends Error {
  constructor(cause?: Error) {
    super('No camera found on this device.');
    this.name = 'NoCameraError';
    this.cause = cause;
  }
}

/**
 * Default camera provider using qr-scanner library
 */
class QrScannerCameraProvider implements CameraProvider {
  private scanner: QrScannerLib | null = null;

  async getCameras(): Promise<CameraDevice[]> {
    const cameras = await QrScannerLib.listCameras(true);
    return cameras.map(cam => ({ id: cam.id, label: cam.label }));
  }

  async start(
    videoElement: HTMLVideoElement,
    cameraId: string,
    onSuccess: (result: string) => void
  ): Promise<void> {
    this.scanner = new QrScannerLib(
      videoElement,
      (result) => {
        onSuccess(result.data);
      },
      {
        preferredCamera: cameraId,
        highlightScanRegion: true,
        highlightCodeOutline: true,
        // Scan the entire video frame for better detection on low-res webcams
        calculateScanRegion: (video) => ({
          x: 0,
          y: 0,
          width: video.videoWidth,
          height: video.videoHeight,
        }),
      }
    );
    await this.scanner.start();
  }

  async stop(): Promise<void> {
    if (this.scanner) {
      this.scanner.stop();
      this.scanner.destroy();
      this.scanner = null;
    }
  }

  isScanning(): boolean {
    return this.scanner !== null;
  }
}

/**
 * QrScanner - Inline QR code scanner component
 *
 * Provides camera-based QR code scanning with expandable UI.
 * Emits 'qr-scanned' event when a QR code is successfully decoded.
 *
 * @fires qr-scanned - Fired when QR code is scanned, detail: { rawValue: string }
 * @fires scanner-error - Fired when scanner encounters an error, detail: ScannerErrorEventDetail
 *
 * Usage:
 * ```html
 * <qr-scanner @qr-scanned=${this._handleQrScanned}></qr-scanner>
 * ```
 */
export class QrScanner extends LitElement {
  // Disable Shadow DOM - qr-scanner library doesn't support it
  // (checks document.body.contains(video) which fails in Shadow DOM)
  override createRenderRoot() {
    return this;
  }

  static override styles = [
    foundationCSS,
    buttonCSS,
    responsiveCSS,
    css`
      :host {
        display: block;
      }

      .scanner-container {
        margin-top: 8px;
      }

      .scanner-toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 14px;
        background: #f5f5f5;
        border: 1px solid #ddd;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
        color: #333;
        width: 100%;
        text-align: left;
      }

      .scanner-toggle:hover:not(:disabled) {
        background: #e8e8e8;
      }

      .scanner-toggle:disabled {
        cursor: not-allowed;
        opacity: 0.6;
      }

      .scanner-toggle .icon {
        font-size: 16px;
      }

      .scanner-area {
        margin-top: 12px;
        border: 1px solid #ddd;
        border-radius: 4px;
        overflow: hidden;
        background: #000;
      }

      .scanner-area.collapsed {
        display: none;
      }

      .viewfinder-container {
        position: relative;
        width: 100%;
        min-height: 250px;
      }

      #qr-video {
        display: block !important;
        width: 100% !important;
        min-width: 100% !important;
        height: 250px !important;
        min-height: 250px !important;
        object-fit: cover;
        visibility: visible !important;
      }

      .viewfinder-container video {
        display: block !important;
        width: 100% !important;
        height: 250px !important;
        visibility: visible !important;
      }

      .scanner-controls {
        display: flex;
        justify-content: center;
        padding: 12px;
        background: #1a1a1a;
        gap: 12px;
      }

      .stop-button {
        padding: 8px 16px;
        background: #dc3545;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 14px;
      }

      .stop-button:hover {
        background: #c82333;
      }

      .error-message {
        padding: 12px;
        background: #fef2f2;
        border: 1px solid #fecaca;
        border-radius: 4px;
        color: #dc2626;
        font-size: 14px;
        margin-top: 8px;
      }

      .loading {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 40px;
        color: #ccc;
        font-size: 14px;
      }

      .camera-select {
        padding: 12px;
        background: #f0f0f0;
        border-bottom: 1px solid #ddd;
      }

      .camera-select label {
        display: block;
        margin-bottom: 6px;
        font-size: 12px;
        color: #666;
      }

      .camera-select select {
        width: 100%;
        padding: 8px;
        border: 1px solid #ccc;
        border-radius: 4px;
        font-size: 14px;
      }
    `,
  ];

  @property({ type: Boolean, reflect: true })
  expanded = false;

  /** When true, hides built-in controls (toggle, camera select, stop button) - parent handles UI */
  @property({ type: Boolean })
  embedded = false;

  @state()
  private scanning = false;

  @state()
  private loading = false;

  @state()
  private error?: Error;

  @state()
  private cameras: CameraDevice[] = [];

  @state()
  private selectedCameraId?: string;

  private cameraProvider: CameraProvider = new QrScannerCameraProvider();

  /**
   * Set a custom camera provider (for testing)
   */
  setCameraProvider(provider: CameraProvider): void {
    this.cameraProvider = provider;
  }

  /**
   * Expand the scanner UI and start camera
   */
  async expand(): Promise<void> {
    this.expanded = true;
    this.error = undefined;
    this.loading = true;

    try {
      this.cameras = await this.cameraProvider.getCameras();
      if (this.cameras.length === 0) {
        throw new NoCameraError();
      }

      // Prefer back camera on mobile
      const backCamera = this.cameras.find(c =>
        c.label.toLowerCase().includes('back') ||
        c.label.toLowerCase().includes('rear') ||
        c.label.toLowerCase().includes('environment')
      );
      this.selectedCameraId = backCamera?.id || this.cameras[0].id;

      // Wait for DOM update before starting scanner
      await this.updateComplete;
      await this._startScanning();
    } catch (err) {
      console.error('[QrScanner] Error in expand():', err);
      this._handleError(err);
    } finally {
      this.loading = false;
    }
  }

  /**
   * Collapse the scanner UI and stop camera
   */
  async collapse(): Promise<void> {
    await this._stopScanning();
    this.expanded = false;
    this.error = undefined;
    this.cameras = [];
    this.selectedCameraId = undefined;
  }

  /**
   * Toggle the scanner expanded state
   */
  async toggle(): Promise<void> {
    if (this.expanded) {
      await this.collapse();
    } else {
      await this.expand();
    }
  }

  override disconnectedCallback(): void {
    super.disconnectedCallback();
    this._stopScanning();
  }

  private async _startScanning(): Promise<void> {
    if (!this.selectedCameraId) {
      console.warn('[QrScanner] No camera selected, aborting');
      return;
    }

    const videoElement = this.querySelector('#qr-video') as HTMLVideoElement;
    if (!videoElement) {
      console.error('[QrScanner] Video element not found in DOM');
      return;
    }

    try {
      await this.cameraProvider.start(
        videoElement,
        this.selectedCameraId,
        this._onScanSuccess.bind(this)
      );
      this.scanning = true;

      // The library may have moved the video to document.body
      // Move it back into our container and style it
      const container = this.querySelector('.viewfinder-container') as HTMLElement;
      if (container && videoElement.parentElement !== container) {
        container.appendChild(videoElement);
      }

      // Force video to be visible with proper positioning
      videoElement.style.cssText = 'display: block !important; width: 100% !important; height: 250px !important; visibility: visible !important; position: relative !important; object-fit: cover !important;';
    } catch (err) {
      console.error('[QrScanner] Error starting camera:', err);
      this._handleError(err);
    }
  }

  private async _stopScanning(): Promise<void> {
    if (this.cameraProvider.isScanning()) {
      try {
        await this.cameraProvider.stop();
      } catch {
        // Silently ignore stop errors - cleanup operation with no UI feedback path
      }
    }
    this.scanning = false;
  }

  /**
   * Called when QR code is successfully scanned
   * Public for testing purposes
   */
  public _onScanSuccess(decodedText: string): void {
    // Emit custom event
    const event = new CustomEvent<QrScannedEventDetail>('qr-scanned', {
      detail: { rawValue: decodedText },
      bubbles: true,
      composed: true,
    });
    this.dispatchEvent(event);

    // Auto-collapse after successful scan
    this.collapse();
  }

  private _handleError(err: unknown): void {
    let error: Error;

    if (err instanceof CameraPermissionError || err instanceof NoCameraError) {
      // Already a custom error type
      error = err;
    } else if (err instanceof DOMException && (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError')) {
      // Browser permission denied - use DOMException.name (stable API)
      error = new CameraPermissionError(err);
    } else if (typeof err === 'string' && err.toLowerCase().includes('no camera')) {
      // qr-scanner library can throw string errors for missing cameras
      error = new NoCameraError(new Error(err));
    } else if (err instanceof Error) {
      // Preserve other Error types
      error = err;
    } else {
      error = new Error('An unknown error occurred');
    }

    this.error = error;
    this.scanning = false;

    // Emit error event with full error object
    const event = new CustomEvent<ScannerErrorEventDetail>('scanner-error', {
      detail: { error },
      bubbles: true,
      composed: true,
    });
    this.dispatchEvent(event);
  }

  private async _handleCameraChange(e: Event): Promise<void> {
    const select = e.target as HTMLSelectElement;
    const newCameraId = select.value;

    // Only restart if camera actually changed
    if (newCameraId === this.selectedCameraId) {
      return;
    }

    this.selectedCameraId = newCameraId;

    await this._stopScanning();
    await this.updateComplete; // Wait for DOM update
    await this._startScanning();
  }

  private _handleStopClick(): void {
    this.collapse();
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="scanner-container">
        ${!this.embedded
          ? html`
              <button
                class="scanner-toggle"
                part="toggle"
                @click=${this.toggle}
                ?disabled=${this.loading}
              >
                <span class="icon"><i class="fa-solid fa-qrcode"></i></span>
                ${this.expanded ? 'Close Scanner' : 'Scan QR Code'}
              </button>
            `
          : nothing}

        ${this.error
          ? html`<div class="error-message" style="padding: 12px; background: #fef2f2; border: 1px solid #fecaca; border-radius: 4px; color: #dc2626; font-size: 14px; margin-top: 8px;">${this.error.message}</div>`
          : nothing}

        <div class="scanner-area ${this.expanded || this.embedded ? '' : 'collapsed'}" part="scanner-area" style="${this.expanded || this.embedded ? 'display: block;' : 'display: none;'}">
          ${this.loading
            ? html`<div class="loading" style="display: flex; align-items: center; justify-content: center; padding: 40px; color: #ccc; font-size: 14px; background: #000;">Starting camera...</div>`
            : nothing}

          <div class="viewfinder-container" style="position: relative; width: 100%; min-height: 250px; background: #000;">
            <video id="qr-video"></video>
          </div>

          ${!this.embedded
            ? html`
                ${this.cameras.length > 1
                  ? html`
                      <div class="camera-select" style="padding: 8px 12px; background: #1a1a1a;">
                        <label for="camera-select" style="color: #ccc; font-size: 12px; margin-right: 8px;">Camera</label>
                        <select
                          id="camera-select"
                          .value=${this.selectedCameraId || ''}
                          @change=${this._handleCameraChange}
                          style="padding: 4px 8px; border-radius: 4px;"
                        >
                          ${this.cameras.map(
                            camera => html`
                              <option value=${camera.id}>${camera.label}</option>
                            `
                          )}
                        </select>
                      </div>
                    `
                  : nothing}

                <div class="scanner-controls" style="display: flex; justify-content: center; padding: 12px; background: #1a1a1a; gap: 12px;">
                  <button class="stop-button" @click=${this._handleStopClick} style="padding: 8px 16px; background: #dc3545; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 14px;">
                    Stop Scanning
                  </button>
                </div>
              `
            : nothing}
        </div>
      </div>
    `;
  }
}

customElements.define('qr-scanner', QrScanner);

declare global {
  interface HTMLElementTagNameMap {
    'qr-scanner': QrScanner;
  }
}
