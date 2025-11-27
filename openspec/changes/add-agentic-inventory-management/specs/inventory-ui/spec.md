# Inventory UI

User interface for managing inventory items via modal dialogs in the wiki toolbar.

## ADDED Requirements

### Requirement: Inventory Submenu Visibility

The system SHALL display an "Inventory" submenu under the Tools menu only when the current page has inventory data in its frontmatter.

#### Scenario: Page with inventory frontmatter
- **WHEN** a user views a page that has `inventory` in its frontmatter
- **THEN** the Tools menu displays an "Inventory" submenu with actions

#### Scenario: Page without inventory frontmatter
- **WHEN** a user views a page that does not have `inventory` in its frontmatter
- **THEN** no Inventory submenu is displayed

### Requirement: Add Item Action

The system SHALL provide an "Add Item Here" action that creates a new inventory item in the current container.

#### Scenario: Open add item dialog
- **WHEN** a user selects "Add Item Here" from the Inventory submenu
- **THEN** a modal dialog opens with fields for item identifier and optional title
- **AND** the container field is pre-populated with the current page identifier (readonly)

#### Scenario: Successful item creation
- **WHEN** a user submits valid item data in the Add Item dialog
- **THEN** the system calls `CreateInventoryItem` via gRPC
- **AND** displays a success message with the created item name
- **AND** closes the dialog

#### Scenario: Item creation error
- **WHEN** a user submits item data that causes an error (e.g., duplicate identifier)
- **THEN** the dialog displays the error message
- **AND** remains open for the user to correct the input

### Requirement: Move Item Action

The system SHALL provide a "Move This Item" action that moves the current item to a different container.

#### Scenario: Open move item dialog
- **WHEN** a user selects "Move This Item" from the Inventory submenu
- **THEN** a modal dialog opens showing the current container (readonly)
- **AND** provides a text input for the destination container identifier

#### Scenario: Successful item move
- **WHEN** a user enters a valid destination and confirms the move
- **THEN** the system calls `MoveInventoryItem` via gRPC
- **AND** displays a success message showing the move path (e.g., "Moved to: drawer_kitchen")
- **AND** closes the dialog

#### Scenario: Item move error
- **WHEN** a user attempts a move that causes an error
- **THEN** the dialog displays the error message
- **AND** remains open for the user to correct the input

### Requirement: Find Item Action

The system SHALL provide a "Find Item" action that searches for an item's location.

#### Scenario: Open find item dialog
- **WHEN** a user selects "Find Item" from the Inventory submenu
- **THEN** a modal dialog opens with a text input for item identifier search

#### Scenario: Item found in single container
- **WHEN** a user searches for an item that exists in one container
- **THEN** the dialog displays the container path with a clickable link to the container page

#### Scenario: Item found in multiple containers
- **WHEN** a user searches for an item that exists in multiple containers
- **THEN** the dialog displays all locations with clickable links
- **AND** shows an anomaly warning indicating the item appears in multiple containers

#### Scenario: Item not found
- **WHEN** a user searches for an item that has no container
- **THEN** the dialog displays a message indicating the item has no container assignment
