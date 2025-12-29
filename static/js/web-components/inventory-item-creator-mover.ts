import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { InventoryManagementService, CreateInventoryItemRequestSchema, MoveInventoryItemRequestSchema } from '../gen/api/v1/inventory_pb.js';
import { PageManagementService, GenerateIdentifierRequestSchema, type ExistingPageInfo } from '../gen/api/v1/page_management_pb.js';
import { AugmentErrorService } from './augment-error-service.js';
import { showToastAfter } from './toast-message.js';

const SUCCESS_TOAST_DURATION_SECONDS = 5;
const ERROR_TOAST_DURATION_SECONDS = 8;

/**
 * InventoryItemCreatorMover - Creates and moves inventory items via gRPC APIs
 *
 * This class manages inventory operations:
 * 1. Add Item Here - Creates a new item in a container
 * 2. Move This Item - Moves an item to a different container
 *
 * Usage:
 * ```typescript
 * const creatorMover = new InventoryItemCreatorMover();
 * creatorMover.addItem('drawer_kitchen', 'screwdriver');
 * creatorMover.moveItem('screwdriver', 'toolbox');
 * ```
 */
export class InventoryItemCreatorMover {
  private inventoryClient = createClient(InventoryManagementService, getGrpcWebTransport());
  private pageManagementClient = createClient(PageManagementService, getGrpcWebTransport());

  /**
   * Creates a new inventory item in a container
   * @param containerIdentifier The container to add the item to
   * @param itemIdentifier The identifier for the new item
   * @param title Optional title for the item
   * @param description Optional description for the item
   */
  async addItem(
    containerIdentifier: string,
    itemIdentifier: string,
    title?: string,
    description?: string
  ): Promise<{ success: boolean; itemIdentifier?: string; summary?: string; error?: Error }> {
    if (!containerIdentifier || !itemIdentifier) {
      return { success: false, error: new Error('Container and item identifier are required') };
    }

    try {
      const request = create(CreateInventoryItemRequestSchema, {
        itemIdentifier,
        container: containerIdentifier,
        title: title || '',
        description: description || '',
      });

      const response = await this.inventoryClient.createInventoryItem(request);

      if (response.success) {
        return {
          success: true,
          itemIdentifier: response.itemIdentifier,
          summary: response.summary,
        };
      } else {
        return {
          success: false,
          error: new Error(response.error || 'Failed to create item'),
        };
      }
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'create inventory item');
      return {
        success: false,
        error: augmentedError,
      };
    }
  }

  /**
   * Generates a wiki identifier from text
   * @param text The text to convert to an identifier
   * @param ensureUnique If true, appends suffix to ensure no page exists with this identifier
   * @returns The generated identifier and availability info
   */
  async generateIdentifier(
    text: string,
    ensureUnique = false
  ): Promise<{
    identifier: string;
    isUnique: boolean;
    existingPage?: ExistingPageInfo;
    error?: Error;
  }> {
    if (!text) {
      return { identifier: '', isUnique: true };
    }

    try {
      const request = create(GenerateIdentifierRequestSchema, {
        text,
        ensureUnique,
      });

      const response = await this.pageManagementClient.generateIdentifier(request);

      return {
        identifier: response.identifier,
        isUnique: response.isUnique,
        existingPage: response.existingPage,
      };
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'generate identifier');
      return {
        identifier: '',
        isUnique: true,
        error: augmentedError,
      };
    }
  }

  /**
   * Moves an item to a new container
   * @param itemIdentifier The item to move
   * @param newContainer The destination container
   */
  async moveItem(
    itemIdentifier: string,
    newContainer: string
  ): Promise<{
    success: boolean;
    previousContainer?: string;
    newContainer?: string;
    summary?: string;
    error?: Error;
  }> {
    if (!itemIdentifier) {
      return { success: false, error: new Error('Item identifier is required') };
    }

    try {
      const request = create(MoveInventoryItemRequestSchema, {
        itemIdentifier,
        newContainer,
      });

      const response = await this.inventoryClient.moveInventoryItem(request);

      if (response.success) {
        return {
          success: true,
          previousContainer: response.previousContainer,
          newContainer: response.newContainer,
          summary: response.summary,
        };
      } else {
        return {
          success: false,
          error: new Error(response.error || 'Failed to move item'),
        };
      }
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'move inventory item');
      return {
        success: false,
        error: augmentedError,
      };
    }
  }

  /**
   * Shows a success toast message
   */
  showSuccess(message: string, callback?: () => void) {
    showToastAfter(message, 'success', SUCCESS_TOAST_DURATION_SECONDS, callback);
  }

  /**
   * Shows an error toast message
   */
  showError(message: string) {
    showToastAfter(message, 'error', ERROR_TOAST_DURATION_SECONDS);
  }
}
