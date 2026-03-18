import type { InventoryAddItemDialog } from './inventory-add-item-dialog.js';
import type { InventoryMoveItemDialog } from './inventory-move-item-dialog.js';

export interface InventoryData {
  inventory: { items?: unknown; container?: string } | null | undefined;
  isContainer: boolean;
  isItem: boolean;
  currentContainer: string;
}

export function extractInventoryData(frontmatter: unknown): InventoryData {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- extracting nested object from unknown frontmatter
  const inventory = (frontmatter && typeof frontmatter === 'object')
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- extracting inventory property
    ? (frontmatter as Record<string, unknown>)['inventory'] as { items?: unknown; container?: string } | null | undefined
    : null;
  const isContainer = !!(inventory && (Array.isArray(inventory.items) || inventory.items !== undefined));
  const isItem = !!(inventory && typeof inventory.container === 'string' && inventory.container !== '');
  const currentContainer = (inventory && inventory.container) ?? '';

  return { inventory, isContainer, isItem, currentContainer };
}

export function validateInventoryResponse(data: unknown): boolean {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- validating unknown data structure
  return !!(data && typeof data === 'object' && Array.isArray((data as Record<string, unknown>)['ids']));
}

export function findPageInInventoryList(ids: unknown[], currentPage: string): boolean {
  return ids.some(function(item) {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- checking identifier property on unknown item
    return item && (item as Record<string, unknown>)['identifier'] === currentPage;
  });
}

function buildInventoryMenu(currentPage: string, frontmatter: unknown): void {
  const utilitySection = document.getElementById('utilityMenuSection');
  if (!utilitySection) {
    return;
  }

  const { isContainer, isItem, currentContainer } = extractInventoryData(frontmatter);

  const addItemHtml = isContainer ? `
    <li class="pure-menu-item">
      <a href="#" class="pure-menu-link" id="inventory-add-item"><i class="fa-solid fa-plus"></i> Add Item Here</a>
    </li>
  ` : '';

  const moveItemHtml = isItem ? `
    <li class="pure-menu-item">
      <a href="#" class="pure-menu-link" id="inventory-move-item"><i class="fa-solid fa-arrows-up-down-left-right"></i> Move This Item</a>
    </li>
  ` : '';

  const inventoryMenuHtml = `
    <li class="pure-menu-item pure-menu-has-children" id="inventory-submenu">
      <a href="#" class="pure-menu-link" id="inventory-submenu-trigger"><i class="fa-solid fa-box-open"></i> Inventory</a>
      <ul class="pure-menu-children">
        ${addItemHtml}
        ${moveItemHtml}
      </ul>
    </li>
  `;

  utilitySection.insertAdjacentHTML('afterend', inventoryMenuHtml);

  const submenu = document.getElementById('inventory-submenu');
  const trigger = document.getElementById('inventory-submenu-trigger');

  trigger?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    submenu?.classList.toggle('submenu-open');
  });

  document.addEventListener('click', (e) => {
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- e.target in DOM click events is always a Node
    if (submenu && !submenu.contains(e.target as Node)) {
      submenu.classList.remove('submenu-open');
    }
  });

  if (isContainer) {
    const addItemEl = document.getElementById('inventory-add-item');
    addItemEl?.addEventListener('click', (e) => {
      e.preventDefault();
      submenu?.classList.remove('submenu-open');
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- inventory-add-item-dialog is registered in HTMLElementTagNameMap
      const dialog = document.getElementById('inventory-add-dialog') as InventoryAddItemDialog | null;
      dialog?.openDialog(currentPage);
    });
  }

  if (isItem) {
    const moveItemEl = document.getElementById('inventory-move-item');
    moveItemEl?.addEventListener('click', (e) => {
      e.preventDefault();
      submenu?.classList.remove('submenu-open');
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- inventory-move-item-dialog is registered in HTMLElementTagNameMap
      const dialog = document.getElementById('inventory-move-dialog') as InventoryMoveItemDialog | null;
      dialog?.openDialog(currentPage, currentContainer);
    });
  }
}

export function initInventoryMenu(): void {
  const contentEl = document.querySelector('article.content');
  if (!contentEl) {
    return;
  }

  const currentPage = contentEl.id;
  if (!currentPage) {
    return;
  }

  fetch('/api/find_by_key_existence?k=inventory')
    .then(response => {
      if (!response.ok) {
        throw new Error(`Failed to check inventory: ${response.status}`);
      }
      return response.json() as Promise<unknown>;
    })
    .then(data => {
      if (!validateInventoryResponse(data)) {
        return;
      }

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- validateInventoryResponse confirms ids is an array
      const { ids } = data as { ids: unknown[] };
      if (!findPageInInventoryList(ids, currentPage)) {
        return;
      }

      return fetch(`/${encodeURIComponent(currentPage)}/frontmatter`)
        .then(response => {
          if (!response.ok) {
            return {} as unknown;
          }
          return response.json() as Promise<unknown>;
        })
        .then(frontmatter => {
          buildInventoryMenu(currentPage, frontmatter);
        })
        .catch((error: unknown) => {
          console.error('Error fetching frontmatter:', error);
          buildInventoryMenu(currentPage, {});
        });
    })
    .catch((error: unknown) => {
      console.error('Error checking inventory:', error);
    });
}
