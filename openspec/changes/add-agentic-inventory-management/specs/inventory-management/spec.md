# Inventory Management

Agentic API for managing physical household inventory items in the wiki.

## ADDED Requirements

### Requirement: Create Inventory Item

The system SHALL provide a `CreateInventoryItem` RPC that creates a new inventory item page with proper frontmatter and template structure.

#### Scenario: Create item in a container
- **WHEN** an agent calls `CreateInventoryItem` with an item identifier and container identifier
- **THEN** a new page is created using the `inv_item` template
- **AND** the page frontmatter includes `inventory.container` set to the container identifier
- **AND** the page frontmatter includes `inventory.items` as an empty array
- **AND** the page title is auto-generated from the identifier if not provided

#### Scenario: Create item with custom title
- **WHEN** an agent calls `CreateInventoryItem` with an item identifier, container, and title
- **THEN** the page frontmatter includes the provided title

#### Scenario: Item already exists
- **WHEN** an agent calls `CreateInventoryItem` for an item that already has a page
- **THEN** the request fails with an appropriate error indicating the item exists

### Requirement: Move Inventory Item

The system SHALL provide a `MoveInventoryItem` RPC that moves an item to a new container, ensuring the item has its own page.

#### Scenario: Move item that has a page
- **WHEN** an agent calls `MoveInventoryItem` for an item that has a wiki page
- **THEN** the item's `inventory.container` is updated to the new container identifier
- **AND** the item is removed from any `inventory.items` lists it appears in

#### Scenario: Move item that only exists in inventory.items
- **WHEN** an agent calls `MoveInventoryItem` for an item that only exists in a container's `inventory.items` list
- **THEN** a page is created for the item using the `inv_item` template
- **AND** the new page's `inventory.container` is set to the destination container
- **AND** the item is removed from the source container's `inventory.items` list

### Requirement: List Container Contents

The system SHALL provide a `ListContainerContents` RPC that returns all items in a container, deduplicating and preferring items with pages.

#### Scenario: List direct contents
- **WHEN** an agent calls `ListContainerContents` for a container
- **THEN** all items with `inventory.container` pointing to this container are returned
- **AND** all items in the container's `inventory.items` list are returned

#### Scenario: Deduplicate items with pages
- **WHEN** an item appears both in the container's `inventory.items` list and has its own page with `inventory.container` set
- **THEN** the item appears only once in the result
- **AND** the item data is taken from the page (not just the `inventory.items` entry)

#### Scenario: List nested contents
- **WHEN** an agent calls `ListContainerContents` with recursive flag
- **THEN** items in nested containers are also returned with their nesting depth

#### Scenario: Empty container
- **WHEN** an agent calls `ListContainerContents` for a container with no items
- **THEN** an empty list is returned

### Requirement: Find Item Location

The system SHALL provide a `FindItemLocation` RPC that finds which container(s) an item is in.

#### Scenario: Item in single container
- **WHEN** an agent calls `FindItemLocation` for an item
- **THEN** the container identifier is returned

#### Scenario: Item in multiple containers (anomaly)
- **WHEN** an agent calls `FindItemLocation` for an item that appears in multiple containers
- **THEN** all container identifiers are returned
- **AND** the response indicates this is an anomaly

#### Scenario: Item not in any container
- **WHEN** an agent calls `FindItemLocation` for an orphaned item
- **THEN** an empty result is returned indicating the item has no container

### Requirement: Inventory Normalization Job

The system SHALL run a scheduled background job that normalizes inventory data, fixing what it can and reporting anomalies it cannot fix.

#### Scenario: Create pages for items without pages
- **WHEN** the normalization job runs
- **AND** an item exists in a container's `inventory.items` list but has no wiki page
- **THEN** a page is created for the item using the `inv_item` template
- **AND** the page's `inventory.container` is set to the listing container

#### Scenario: Remove duplicates from inventory.items
- **WHEN** the normalization job runs
- **AND** an item has its own page with `inventory.container` set
- **AND** the same item also appears in a container's `inventory.items` list
- **THEN** the item is removed from the `inventory.items` list
- **AND** the page's `inventory.container` is treated as the source of truth

#### Scenario: Detect items in multiple containers
- **WHEN** the normalization job runs
- **AND** an item's page has `inventory.container` set to one container
- **AND** the same item appears in a different container's `inventory.items` list
- **THEN** the item is removed from the `inventory.items` list (page's `.container` wins)

#### Scenario: Report unresolvable anomalies
- **WHEN** the normalization job runs
- **AND** an anomaly cannot be automatically fixed (e.g., circular container references)
- **THEN** this anomaly is recorded in the audit report wiki page

#### Scenario: Update audit report page
- **WHEN** the normalization job completes
- **THEN** the designated audit report wiki page is updated with current anomalies
- **AND** previously reported anomalies that are resolved are removed from the report
