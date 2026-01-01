/**
 * Shared event detail type definitions for custom events.
 *
 * This file provides type definitions for custom events dispatched and consumed
 * by various web components. When a component needs to emit or handle an event
 * with a typed detail, it should either:
 * 1. Export the event detail interface from the emitting component
 * 2. Use this shared file when circular dependencies would occur
 */

import type { JsonObject } from '@bufbuild/protobuf';

/**
 * Event detail for page-created event from InsertNewPageDialog.
 * @fires page-created - Dispatched when a new page is successfully created.
 */
export interface PageCreatedEventDetail {
  identifier: string;
  title: string;
  markdownLink: string;
}

/**
 * Event detail for title-change event from AutomagicIdentifierInput.
 * @fires title-change - Dispatched when the title input value changes.
 */
export interface TitleChangeEventDetail {
  title: string;
}

/**
 * Event detail for identifier-change event from AutomagicIdentifierInput.
 * @fires identifier-change - Dispatched when the identifier changes.
 */
export interface IdentifierChangeEventDetail {
  identifier: string;
  isUnique: boolean;
}

/**
 * Event detail for section-change event from FrontmatterValueSection.
 * @fires section-change - Dispatched when frontmatter fields are modified.
 */
export interface SectionChangeEventDetail {
  oldFields: JsonObject;
  newFields: JsonObject;
}

/**
 * Event detail for inventory-filter-changed event from WikiSearchResults.
 * @fires inventory-filter-changed - Dispatched when the inventory-only filter is toggled.
 */
export interface InventoryFilterChangedEventDetail {
  inventoryOnly: boolean;
}
