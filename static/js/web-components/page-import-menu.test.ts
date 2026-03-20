import { expect } from '@esm-bundle/chai';
import sinon from 'sinon';
import { initPageImportMenu } from './page-import-menu.js';

describe('initPageImportMenu', () => {
  let utilitySection: HTMLElement;

  beforeEach(() => {
    utilitySection = document.createElement('hr');
    utilitySection.id = 'utilityMenuSection';
    document.body.appendChild(utilitySection);
  });

  afterEach(() => {
    utilitySection.remove();
    document.querySelectorAll('.pure-menu-item').forEach(el => el.remove());
    document.getElementById('page-import-trigger')?.remove();
    sinon.restore();
  });

  describe('when utilityMenuSection is absent', () => {
    beforeEach(() => {
      utilitySection.remove();
      initPageImportMenu();
    });

    it('should not inject any menu items', () => {
      expect(document.querySelectorAll('.pure-menu-item')).to.have.lengthOf(0);
    });
  });

  describe('when utilityMenuSection is present', () => {
    beforeEach(() => {
      initPageImportMenu();
    });

    it('should inject a menu item', () => {
      expect(document.querySelectorAll('.pure-menu-item')).to.have.lengthOf(1);
    });

    it('should inject a link with id page-import-trigger', () => {
      const link = document.getElementById('page-import-trigger');
      expect(link).to.exist;
    });

    it('should include a FontAwesome import icon', () => {
      const icon = document.querySelector('#page-import-trigger i.fa-solid.fa-file-import');
      expect(icon).to.exist;
    });

    it('should include the text " Import Pages"', () => {
      const link = document.getElementById('page-import-trigger');
      expect(link?.textContent).to.include('Import Pages');
    });

    it('should have href="#" on the link', () => {
      const link = document.getElementById('page-import-trigger') as HTMLAnchorElement | null;
      expect(link?.href).to.include('#');
    });

    it('should apply pure-menu-link class to the link', () => {
      const link = document.getElementById('page-import-trigger');
      expect(link?.classList.contains('pure-menu-link')).to.be.true;
    });

    it('should apply pure-menu-item class to the list item', () => {
      const item = document.querySelector('.pure-menu-item');
      expect(item).to.exist;
    });
  });

  describe('when the import link is clicked', () => {
    let openDialogStub: sinon.SinonStub;
    let dialog: HTMLElement & { openDialog?: () => void };

    beforeEach(() => {
      // Add a mock page-import-dialog element
      dialog = document.createElement('div') as HTMLElement & { openDialog?: () => void };
      dialog.id = 'page-import-dialog';
      openDialogStub = sinon.stub();
      dialog.openDialog = openDialogStub;
      document.body.appendChild(dialog);

      initPageImportMenu();
      const link = document.getElementById('page-import-trigger') as HTMLAnchorElement;
      link.click();
    });

    afterEach(() => {
      dialog.remove();
    });

    it('should call openDialog on the page-import-dialog element', () => {
      expect(openDialogStub.calledOnce).to.be.true;
    });
  });

  describe('when the import link is clicked and dialog is absent', () => {
    beforeEach(() => {
      // Ensure no dialog element exists
      document.getElementById('page-import-dialog')?.remove();
      initPageImportMenu();
      // Click should not throw even though dialog is absent
      const link = document.getElementById('page-import-trigger') as HTMLAnchorElement;
      link.click();
    });

    it('should not inject any additional menu items (menu was already built)', () => {
      // If we reach this assertion, the click handler did not throw
      expect(document.querySelectorAll('.pure-menu-item')).to.have.lengthOf(1);
    });
  });

  describe('when the import link default action is triggered', () => {
    let clickEvent: MouseEvent;
    let defaultPrevented: boolean;

    beforeEach(() => {
      initPageImportMenu();
      const link = document.getElementById('page-import-trigger') as HTMLAnchorElement;
      clickEvent = new MouseEvent('click', { bubbles: true, cancelable: true });
      defaultPrevented = false;
      link.addEventListener('click', (e) => { defaultPrevented = e.defaultPrevented; }, { once: true });
      link.dispatchEvent(clickEvent);
    });

    it('should prevent the default link navigation', () => {
      expect(defaultPrevented).to.be.true;
    });
  });
});
