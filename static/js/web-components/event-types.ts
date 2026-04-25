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

/**
 * Event detail for add-field event from FrontmatterAddFieldButton.
 * @fires add-field - Dispatched when the user selects a field type to add.
 */
export interface AddFieldEventDetail {
  type: 'field' | 'array' | 'section';
}

/**
 * Event detail for wiki-search-open event from WikiHashtag (and any other
 * surface that wants to invoke the popup search). Dispatched on window —
 * `<wiki-search>` listens at window level and runs the supplied query
 * immediately. Lives here because it's consumed by both wiki-search and
 * wiki-hashtag, which would otherwise import each other.
 * @fires wiki-search-open - Dispatched to request the global search popup
 * open with a pre-filled query.
 */
export interface WikiSearchOpenEventDetail {
  query: string;
}

declare global {
  // Augment WindowEventMap so addEventListener('wiki-search-open', …)
  // is type-checked with the correct CustomEvent payload.
  interface WindowEventMap {
    'wiki-search-open': CustomEvent<WikiSearchOpenEventDetail>;
  }
}
