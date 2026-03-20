import { showToast } from './toast-message.js';

export async function printLabel(templateIdentifier: string): Promise<void> {
  const contentEl = document.querySelector('article.content');
  const dataIdentifier = contentEl?.id ?? window.simple_wiki?.pageName ?? '';

  try {
    const response = await fetch('/api/print_label', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ template_identifier: templateIdentifier, data_identifier: dataIdentifier }),
    });
    const data = await response.json() as unknown;
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- extracting message from unknown API response
    const message = (data as Record<string, unknown>)['message'];
    showToast(typeof message === 'string' ? message : 'Print successful', 'success', 5);
  } catch (error: unknown) {
    const errorMessage = error instanceof Error ? error.message : 'Print failed';
    showToast(errorMessage, 'error', 5);
  }
}

export function initPrintMenu(): void {
  const contentEl = document.querySelector('article.content');
  if (!contentEl) {
    return;
  }

  fetch('/api/find_by_key_existence?k=label_printer')
    .then(response => {
      if (!response.ok) {
      throw new Error(`Server returned error status when fetching print labels: ${response.status}`);
      }
      return response.json() as Promise<unknown>;
    })
    .then(data => {
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- extracting ids from unknown API response
      const ids = (data as Record<string, unknown>)['ids'];
      if (!Array.isArray(ids)) {
        return;
      }

      const utilitySection = document.getElementById('utilityMenuSection');
      if (!utilitySection) {
        return;
      }

      ids.forEach(item => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- extracting fields from unknown item
        const itemData = item as Record<string, unknown>;
        const identifier = itemData['identifier'];
        const title = itemData['title'];

        if (typeof identifier !== 'string') {
          return;
        }

        const menuItem = document.createElement('li');
        menuItem.className = 'pure-menu-item';
        const link = document.createElement('a');
        link.href = '#';
        link.className = 'pure-menu-link';
        link.textContent = `Print ${typeof title === 'string' ? title : identifier}`;
        link.addEventListener('click', (e) => {
          e.preventDefault();
          void printLabel(identifier);
        });
        menuItem.appendChild(link);
        utilitySection.insertAdjacentElement('afterend', menuItem);
      });
    })
    .catch((_error: unknown) => {
      // Silently ignore: print menu is non-critical, failure just means no print options shown
    });
}
