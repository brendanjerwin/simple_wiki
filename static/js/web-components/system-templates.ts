/**
 * System template identifiers - templates that are used internally
 * and should not be shown in user-facing template selectors.
 */

/** Inventory item template - used for inventory management */
export const INVENTORY_ITEM_TEMPLATE = 'inv_item';

/** List of all system template identifiers to exclude from user selection */
export const SYSTEM_TEMPLATE_IDENTIFIERS: readonly string[] = [
  INVENTORY_ITEM_TEMPLATE,
] as const;
