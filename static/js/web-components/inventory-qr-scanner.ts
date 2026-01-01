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
import './error-display.js';
import type { AugmentedError } from './augment-error-service.js';
import { AugmentErrorService } from './augment-error-service.js';
import type { ErrorAction } from './error-display.js';

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
        border-radius: 8px;
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
  private augmentedError: AugmentedError | null = null;

  @state()
  private validating = false;

  private frontmatterClient = createClient(Frontmatter, getGrpcWebTransport());

  /**
   * Expand the scanner and start camera
   */
  async expand(): Promise<void> {
    this.augmentedError = null;
    await this.updateComplete;
    const scanner = this.shadowRoot?.querySelector<QrScanner>('qr-scanner');
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
    if (this.augmentedError) {
      return;
    }
    const scanner = this.shadowRoot?.querySelector<QrScanner>('qr-scanner');
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
    this.augmentedError = null;

    // Parse the URL
    const parseResult = WikiUrlParser.parse(rawValue);
    if (!parseResult.success || !parseResult.pageIdentifier) {
      this.augmentedError = AugmentErrorService.augmentError(
        new Error(`Not a valid wiki URL: "${rawValue}"`),
        'validating scanned QR code'
      );
      return;
    }

    const identifier = parseResult.pageIdentifier;

    // Fetch page info
    this.validating = true;
    try {
      const request = create(GetFrontmatterRequestSchema, { page: identifier });
      const response = await this.frontmatterClient.getFrontmatter(request);

      // frontmatter is already JsonObject in protobuf-es v2 (no toJson() needed)
      const fm = response.frontmatter;

      // Safely access nested inventory object
      const inventoryRaw = fm?.['inventory'];
      const inventory = typeof inventoryRaw === 'object' && inventoryRaw !== null && !Array.isArray(inventoryRaw)
        ? inventoryRaw
        : undefined;

      // Check if page is a container
      const isContainerValue = inventory?.['is_container'];
      const isContainer = isContainerValue === true || isContainerValue === 'true';

      // Safely extract string values
      const titleRaw = fm?.['title'];
      const title = typeof titleRaw === 'string' ? titleRaw : identifier;

      const containerRaw = inventory?.['container'];
      const container = typeof containerRaw === 'string' ? containerRaw : undefined;

      // Build scanned item info - use conditional spread for optional properties
      const item: ScannedItemInfo = {
        identifier,
        title,
        ...(container !== undefined && { container }),
        ...(isContainerValue !== undefined && { isContainer }),
      };

      // Collapse scanner and emit success event
      await this.collapse();
      this.dispatchEvent(new CustomEvent<ItemScannedEventDetail>('item-scanned', {
        detail: { item },
        bubbles: true,
        composed: true,
      }));
    } catch (err) {
      const errorObj = err instanceof Error ? err : new Error(`Page "${identifier}" not found`);
      this.augmentedError = AugmentErrorService.augmentError(errorObj, 'fetching page info');
    } finally {
      this.validating = false;
    }
  };

  /**
   * Handle "Scan Again" button click
   */
  private _handleScanAgain = async (): Promise<void> => {
    this.augmentedError = null;
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

        ${this.augmentedError ? html`
          <error-display
            .augmentedError=${this.augmentedError}
            .action=${{ label: 'Scan Again', onClick: this._handleScanAgain } satisfies ErrorAction}
          ></error-display>
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
