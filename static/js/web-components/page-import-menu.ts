import type { PageImportDialog } from './page-import-dialog.js';

export function initPageImportMenu(): void {
  const contentEl = document.querySelector('article.content');
  if (!contentEl) {
    return;
  }

  const utilitySection = document.getElementById('utilityMenuSection');
  if (!utilitySection) {
    return;
  }

  const menuItem = document.createElement('li');
  menuItem.className = 'pure-menu-item';
  const link = document.createElement('a');
  link.href = '#';
  link.className = 'pure-menu-link';
  link.id = 'page-import-trigger';
  link.innerHTML = '<i class="fa-solid fa-file-import"></i> Import Pages';
  link.addEventListener('click', (e) => {
    e.preventDefault();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- page-import-dialog is registered in HTMLElementTagNameMap
    const dialog = document.getElementById('page-import-dialog') as PageImportDialog | null;
    dialog?.openDialog();
  });
  menuItem.appendChild(link);
  utilitySection.insertAdjacentElement('afterend', menuItem);
}
