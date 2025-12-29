import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { sharedStyles, foundationCSS, buttonCSS } from './shared-styles.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { Frontmatter, GetFrontmatterRequestSchema } from '../gen/api/v1/frontmatter_pb.js';
import { WikiUrlParser } from '../utils/wiki-url-parser.js';
import './qr-scanner.js';
import type { QrScannedEventDetail, QrScanner } from './qr-scanner.js';

/**
 * Information about a scanned inventory item
 */
export interface ScannedItemInfo {
  identifier: string;
  title: string;
  container?: string;      // The item's parent container
  isContainer?: boolean;   // Whether this item is itself a container
}

/**
 * Event detail for item-scanned event
 */
export interface ItemScannedEventDetail {
  item: ScannedItemInfo;
}

/**
 * InventoryQrScanner - Reusable QR scanner for inventory items
 *
 * Wraps the qr-scanner component to:
 * - Parse scanned wiki URLs
 * - Fetch page info from Frontmatter service
 * - Emit item-scanned event with ScannedItemInfo on success
 * - Handle errors internally with "Scan Again" button
 *
 * @fires item-scanned - Fired when a valid page is scanned, detail: ItemScannedEventDetail
 * @fires cancelled - Fired when user clicks Cancel button
 */
export class InventoryQrScanner extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: block;
      }

      .scanner-container {
        border: 1px solid #ddd;
        border-radius: 4px;
        overflow: hidden;
        background: #000;
      }

      .scanner-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 8px 12px;
        background: #1a1a1a;
        color: #fff;
      }

      .scanner-header .title {
        font-size: 14px;
        font-weight: 500;
      }

      .cancel-button {
        padding: 4px 10px;
        background: #dc3545;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 12px;
      }

      .cancel-button:hover {
        background: #c82333;
      }

      .error-container {
        padding: 12px;
        background: #fef2f2;
        border-top: 1px solid #fecaca;
      }

      .error-message {
        color: #dc2626;
        font-size: 14px;
        margin-bottom: 10px;
        display: flex;
        align-items: center;
        gap: 8px;
      }

      .error-message .icon {
        font-size: 16px;
      }

      .scan-again-button {
        padding: 6px 12px;
        border: 1px solid #fca5a5;
        border-radius: 4px;
        background: white;
        color: #dc2626;
        font-size: 13px;
        cursor: pointer;
        transition: all 0.15s;
      }

      .scan-again-button:hover {
        background: #fef2f2;
      }

      .validating {
        padding: 12px;
        background: #f3f4f6;
        border-top: 1px solid #e5e7eb;
        color: #6b7280;
        font-size: 14px;
        display: flex;
        align-items: center;
        gap: 8px;
      }
    `,
  ];

  @property({ type: Boolean })
  disabled = false;

  @state()
  private error: Error | null = null;

  @state()
  private validating = false;

  private frontmatterClient = createClient(Frontmatter, getGrpcWebTransport());

  /**
   * Expand the scanner and start camera
   */
  async expand(): Promise<void> {
    this.error = null;
    await this.updateComplete;
    const scanner = this.shadowRoot?.querySelector('qr-scanner') as QrScanner | null;
    if (!scanner) {
      throw new Error('InventoryQrScanner: qr-scanner element not found');
    }
    await scanner.expand();
  }

  /**
   * Collapse the scanner and stop camera
   */
  async collapse(): Promise<void> {
    // If error is showing, qr-scanner is not in DOM - nothing to collapse
    if (this.error) {
      return;
    }
    const scanner = this.shadowRoot?.querySelector('qr-scanner') as QrScanner | null;
    if (!scanner) {
      throw new Error('InventoryQrScanner: qr-scanner element not found');
    }
    await scanner.collapse();
  }

  /**
   * Handle Cancel button click
   */
  private _handleCancel = async (): Promise<void> => {
    await this.collapse();
    this.dispatchEvent(new CustomEvent('cancelled', {
      bubbles: true,
      composed: true,
    }));
  };

  /**
   * Handle QR code scanned from inner qr-scanner.
   */
  private _handleQrScanned = async (event: CustomEvent<QrScannedEventDetail>): Promise<void> => {
    const rawValue = event.detail.rawValue;

    // Clear previous error
    this.error = null;

    // Parse the URL
    const parseResult = WikiUrlParser.parse(rawValue);
    if (!parseResult.success || !parseResult.pageIdentifier) {
      this.error = new Error(`Not a valid wiki URL: "${rawValue}"`);
      return;
    }

    const identifier = parseResult.pageIdentifier;

    // Fetch page info
    this.validating = true;
    try {
      const request = create(GetFrontmatterRequestSchema, { page: identifier });
      const response = await this.frontmatterClient.getFrontmatter(request);

      // Convert protobuf Struct to plain object for easy access
      const fm = response.frontmatter?.toJson() as Record<string, unknown> | undefined;
      const inventory = fm?.['inventory'] as Record<string, unknown> | undefined;

      // Check if page is a container
      const isContainerValue = inventory?.['is_container'];
      const isContainer = isContainerValue === true || isContainerValue === 'true';

      // Build scanned item info
      const item: ScannedItemInfo = {
        identifier,
        title: (fm?.['title'] as string) || identifier,
        container: inventory?.['container'] as string | undefined,
        isContainer,
      };

      // Collapse scanner and emit success event
      await this.collapse();
      this.dispatchEvent(new CustomEvent<ItemScannedEventDetail>('item-scanned', {
        detail: { item },
        bubbles: true,
        composed: true,
      }));
    } catch (err) {
      this.error = err instanceof Error ? err : new Error(`Page "${identifier}" not found`);
    } finally {
      this.validating = false;
    }
  };

  /**
   * Handle "Scan Again" button click
   */
  private _handleScanAgain = async (): Promise<void> => {
    this.error = null;
    await this.expand();
  };

  override render() {
    return html`
      ${sharedStyles}
      <div class="scanner-container">
        <div class="scanner-header">
          <span class="title">Scan QR Code</span>
          <button class="cancel-button" @click=${this._handleCancel} ?disabled=${this.disabled}>
            Cancel
          </button>
        </div>

        ${this.error ? html`
          <div class="error-container">
            <div class="error-message">
              <span class="icon"><i class="fa-solid fa-triangle-exclamation"></i></span>
              ${this.error.message}
            </div>
            <button class="scan-again-button" @click=${this._handleScanAgain} ?disabled=${this.disabled}>
              <i class="fa-solid fa-qrcode"></i> Scan Again
            </button>
          </div>
        ` : html`
          <qr-scanner
            embedded
            @qr-scanned=${this._handleQrScanned}
          ></qr-scanner>
          ${this.validating ? html`
            <div class="validating">
              <i class="fa-solid fa-spinner fa-spin"></i>
              Validating scanned page...
            </div>
          ` : nothing}
        `}
      </div>
    `;
  }
}

customElements.define('inventory-qr-scanner', InventoryQrScanner);

declare global {
  interface HTMLElementTagNameMap {
    'inventory-qr-scanner': InventoryQrScanner;
  }
}
