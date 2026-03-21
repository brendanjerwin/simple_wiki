import type { PageImportDialog } from './page-import-dialog.js';

export function initPageImportMenu(): void {
  const utilitySection = document.getElementById('utilityMenuSection');
  if (!utilitySection) {
    return;
  }

  if (!document.body.classList.contains('ViewPage')) {
    return;
  }

  const menuItem = document.createElement('li');
  menuItem.className = 'pure-menu-item';
  const link = document.createElement('a');
  link.href = '#';
  link.className = 'pure-menu-link';
  link.id = 'page-import-trigger';
  link.setAttribute('role', 'menuitem');
  const importIcon = document.createElement('i');
  importIcon.className = 'fa-solid fa-file-import';
  link.appendChild(importIcon);
  link.appendChild(document.createTextNode(' Import Pages'));
  link.addEventListener('click', (e) => {
    e.preventDefault();
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- page-import-dialog is registered in HTMLElementTagNameMap
    const dialog = document.getElementById('page-import-dialog') as PageImportDialog | null;
    dialog?.openDialog();
  });
  menuItem.appendChild(link);
  utilitySection.insertAdjacentElement('afterend', menuItem);
}
