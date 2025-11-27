import { createClient } from '@connectrpc/connect';
import { getGrpcWebTransport } from './grpc-transport.js';
import { InventoryManagementService } from '../gen/api/v1/inventory_connect.js';
import {
  CreateInventoryItemRequest,
  MoveInventoryItemRequest,
  FindItemLocationRequest,
} from '../gen/api/v1/inventory_pb.js';
import { AugmentErrorService } from './augment-error-service.js';
import { showToastAfter } from './toast-message.js';

/**
 * InventoryActionService - Handles inventory management workflows via modal dialogs
 *
 * This service manages inventory operations:
 * 1. Add Item Here - Creates a new item in a container
 * 2. Move This Item - Moves an item to a different container
 * 3. Find Item - Searches for an item's location
 *
 * Usage:
 * ```typescript
 * const service = new InventoryActionService();
 * service.openAddItemDialog('drawer_kitchen');
 * service.openMoveItemDialog('screwdriver', 'drawer_kitchen');
 * service.openFindItemDialog();
 * ```
 */
export class InventoryActionService {
  private client = createClient(InventoryManagementService, getGrpcWebTransport());

  /**
   * Opens the Add Item dialog for a container
   * @param containerIdentifier The container to add the item to
   */
  async addItem(
    containerIdentifier: string,
    itemIdentifier: string,
    title?: string
  ): Promise<{ success: boolean; itemIdentifier?: string; summary?: string; error?: string }> {
    if (!containerIdentifier || !itemIdentifier) {
      return { success: false, error: 'Container and item identifier are required' };
    }

    try {
      const request = new CreateInventoryItemRequest({
        itemIdentifier,
        container: containerIdentifier,
        title: title || '',
      });

      const response = await this.client.createInventoryItem(request);

      if (response.success) {
        return {
          success: true,
          itemIdentifier: response.itemIdentifier,
          summary: response.summary,
        };
      } else {
        return {
          success: false,
          error: response.error || 'Failed to create item',
        };
      }
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'create inventory item');
      return {
        success: false,
        error: augmentedError.message,
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
    error?: string;
  }> {
    if (!itemIdentifier) {
      return { success: false, error: 'Item identifier is required' };
    }

    try {
      const request = new MoveInventoryItemRequest({
        itemIdentifier,
        newContainer,
      });

      const response = await this.client.moveInventoryItem(request);

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
          error: response.error || 'Failed to move item',
        };
      }
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'move inventory item');
      return {
        success: false,
        error: augmentedError.message,
      };
    }
  }

  /**
   * Finds an item's location(s)
   * @param itemIdentifier The item to find
   */
  async findItem(itemIdentifier: string): Promise<{
    success: boolean;
    found?: boolean;
    locations?: Array<{ container: string; path: string[] }>;
    summary?: string;
    error?: string;
  }> {
    if (!itemIdentifier) {
      return { success: false, error: 'Item identifier is required' };
    }

    try {
      const request = new FindItemLocationRequest({
        itemIdentifier,
        includeHierarchy: true,
      });

      const response = await this.client.findItemLocation(request);

      return {
        success: true,
        found: response.found,
        locations: response.locations.map((loc) => ({
          container: loc.container,
          path: loc.path,
        })),
        summary: response.summary,
      };
    } catch (err) {
      const augmentedError = AugmentErrorService.augmentError(err, 'find item location');
      return {
        success: false,
        error: augmentedError.message,
      };
    }
  }

  /**
   * Shows a success toast message
   */
  showSuccess(message: string, callback?: () => void) {
    showToastAfter(message, 'success', 5, callback);
  }

  /**
   * Shows an error toast message
   */
  showError(message: string) {
    showToastAfter(message, 'error', 8);
  }
}

// Create a singleton instance for global use
export const inventoryActionService = new InventoryActionService();
