import { expect } from '@esm-bundle/chai';
import sinon from 'sinon';
import { printLabel, initPrintMenu } from './print-label.js';
import './toast-message.js';
import type { ToastMessage } from './toast-message.js';

function makeJsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function getToast(): ToastMessage | null {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  return document.querySelector('toast-message') as ToastMessage | null;
}

describe('printLabel', () => {
  let fetchStub: sinon.SinonStub;

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch');
    window.simple_wiki = { pageName: 'test_page' };
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  afterEach(() => {
    fetchStub.restore();
    delete window.simple_wiki;
    document.querySelectorAll('toast-message').forEach(el => el.remove());
  });

  describe('when API returns success with message', () => {
    let requestUrl: string;
    let requestMethod: string;
    let requestBody: Record<string, unknown>;

    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ success: true, message: 'Label queued' }));
      await printLabel('my_template');
      requestUrl = fetchStub.firstCall.args[0] as string;
      requestMethod = (fetchStub.firstCall.args[1] as RequestInit).method ?? '';
      requestBody = JSON.parse((fetchStub.firstCall.args[1] as RequestInit).body as string) as Record<string, unknown>;
    });

    it('should POST to /api/print_label', () => {
      expect(fetchStub.calledOnce).to.be.true;
      expect(requestUrl).to.equal('/api/print_label');
      expect(requestMethod).to.equal('POST');
    });

    it('should send template_identifier in body', () => {
      expect(requestBody['template_identifier']).to.equal('my_template');
    });

    it('should send data_identifier from window.simple_wiki.pageName', () => {
      expect(requestBody['data_identifier']).to.equal('test_page');
    });

    it('should show a success toast', () => {
      const toast = getToast();
      expect(toast).to.exist;
      expect(toast!.type).to.equal('success');
    });

    it('should display the message from the response', () => {
      const toast = getToast();
      expect(toast!.message).to.equal('Label queued');
    });
  });

  describe('when API returns success but no message', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ success: true }));
      await printLabel('my_template');
    });

    it('should show a success toast with default message', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('success');
      expect(toast!.message).to.equal('Print successful');
    });
  });

  describe('when API returns success:false', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ success: false, message: 'Printer offline' }));
      await printLabel('my_template');
    });

    it('should show an error toast', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('error');
    });

    it('should display the error message from the response', () => {
      const toast = getToast();
      expect(toast!.message).to.equal('Printer offline');
    });
  });

  describe('when API returns non-2xx status', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ message: 'Internal server error' }, 500));
      await printLabel('my_template');
    });

    it('should show an error toast', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('error');
    });

    it('should display the error message from the response', () => {
      const toast = getToast();
      expect(toast!.message).to.equal('Internal server error');
    });
  });

  describe('when API returns non-2xx status with no message', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({}, 500));
      await printLabel('my_template');
    });

    it('should show an error toast with default message', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('error');
      expect(toast!.message).to.equal('Print failed');
    });
  });

  describe('when fetch throws a network error', () => {
    beforeEach(async () => {
      fetchStub.rejects(new Error('Network error'));
      await printLabel('my_template');
    });

    it('should show an error toast', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('error');
    });

    it('should display the network error message', () => {
      const toast = getToast();
      expect(toast!.message).to.equal('Network error');
    });
  });

  describe('when fetch throws a non-Error object', () => {
    beforeEach(async () => {
      fetchStub.returns(Promise.reject(42));
      await printLabel('my_template');
    });

    it('should show error toast with fallback message', () => {
      const toast = getToast();
      expect(toast!.type).to.equal('error');
      expect(toast!.message).to.equal('Print failed');
    });
  });

  describe('when window.simple_wiki is not set', () => {
    beforeEach(async () => {
      delete window.simple_wiki;
      await printLabel('my_template');
    });

    it('should not make any fetch calls', () => {
      expect(fetchStub.called).to.be.false;
    });

    it('should show an error toast about missing page name', () => {
      const toast = getToast();
      expect(toast).to.exist;
      expect(toast!.type).to.equal('error');
      expect(toast!.message).to.include('page name');
    });
  });
});

describe('initPrintMenu', () => {
  let fetchStub: sinon.SinonStub;
  let utilitySection: HTMLElement;

  beforeEach(() => {
    fetchStub = sinon.stub(window, 'fetch');
    window.simple_wiki = { pageName: 'test_page' };

    // Create the utility menu section expected by the init functions
    utilitySection = document.createElement('hr');
    utilitySection.id = 'utilityMenuSection';
    document.body.appendChild(utilitySection);
  });

  afterEach(() => {
    fetchStub.restore();
    delete window.simple_wiki;
    utilitySection.remove();
    // Remove any menu items injected after the utility section
    document.querySelectorAll('.pure-menu-item').forEach(el => el.remove());
  });

  describe('when utilityMenuSection is absent', () => {
    beforeEach(() => {
      utilitySection.remove();
      initPrintMenu();
    });

    it('should not make any fetch calls', () => {
      expect(fetchStub.called).to.be.false;
    });
  });

  describe('when API returns label printer templates', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({
        ids: [
          { identifier: 'zebra_4x2', title: 'Zebra 4x2' },
          { identifier: 'dymo_standard', title: 'Dymo Standard' },
        ],
      }));
      initPrintMenu();
      // Wait for all microtasks/promise chains to resolve
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should fetch from /api/find_by_key_existence?k=label_printer', () => {
      expect(fetchStub.calledOnce).to.be.true;
      expect(fetchStub.firstCall.args[0]).to.equal('/api/find_by_key_existence?k=label_printer');
    });

    it('should add a menu item for each label printer', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(2);
    });

    it('should insert printer items in the correct order (first API result first)', () => {
      const links = document.querySelectorAll('.pure-menu-link');
      expect(links[0]?.textContent).to.equal('Print Zebra 4x2');
      expect(links[1]?.textContent).to.equal('Print Dymo Standard');
    });

    it('should set role="menuitem" on each link', () => {
      const links = document.querySelectorAll('.pure-menu-link');
      links.forEach(link => {
        expect(link.getAttribute('role')).to.equal('menuitem');
      });
    });

    it('should use the title for the menu item text', () => {
      const links = document.querySelectorAll('.pure-menu-link');
      const texts = Array.from(links).map(l => l.textContent);
      expect(texts).to.include('Print Zebra 4x2');
      expect(texts).to.include('Print Dymo Standard');
    });
  });

  describe('when API returns an item without a title', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({
        ids: [{ identifier: 'no_title_printer' }],
      }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should fall back to the identifier as the link text', () => {
      const links = document.querySelectorAll('.pure-menu-link');
      const texts = Array.from(links).map(l => l.textContent);
      expect(texts).to.include('Print no_title_printer');
    });
  });

  describe('when API returns no items', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ ids: [] }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not add any menu items', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(0);
    });
  });

  describe('when API returns ids that are not an array', () => {
    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({ ids: 'not-array' }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not add any menu items', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(0);
    });
  });

  describe('when API returns a non-OK response', () => {
    beforeEach(async () => {
      fetchStub.resolves(new Response('Server Error', { status: 500 }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not add any menu items', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(0);
    });
  });

  describe('when fetch throws a network error', () => {
    beforeEach(async () => {
      fetchStub.rejects(new Error('Network error'));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should not add any menu items (silently ignores)', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(0);
    });
  });

  describe('when API returns an item with an identifier containing single quotes', () => {
    // Regression guard: the old simple_wiki.js used onclick="printLabel('${identifier}')"
    // which would break or allow injection when identifier contains single quotes.
    // The fix uses addEventListener with a closure, so the identifier is never interpolated.
    const identifierWithQuotes = "it's-a-tricky'identifier";

    beforeEach(async () => {
      fetchStub.resolves(makeJsonResponse({
        ids: [{ identifier: identifierWithQuotes, title: 'Tricky Printer' }],
      }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    it('should still create a menu item', () => {
      const items = document.querySelectorAll('.pure-menu-item');
      expect(items).to.have.lengthOf(1);
    });

    it('should not set an onclick attribute on the link (uses addEventListener instead)', () => {
      const link = document.querySelector('.pure-menu-link');
      expect(link?.getAttribute('onclick')).to.be.null;
    });

    it('should use the title as link text, not the identifier', () => {
      const link = document.querySelector('.pure-menu-link');
      expect(link?.textContent).to.equal('Print Tricky Printer');
    });

    it('should not put the identifier into any HTML attribute', () => {
      const link = document.querySelector('.pure-menu-link');
      expect(link?.outerHTML).to.not.include(identifierWithQuotes);
    });
  });

  describe('when clicking a menu item with a special-character identifier', () => {
    const specialIdentifier = "it's-a-tricky'identifier";

    beforeEach(async () => {
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: specialIdentifier, title: 'Tricky Printer' }],
      }));
      fetchStub.onSecondCall().resolves(makeJsonResponse({ success: true, message: 'Printed' }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));

      const link = document.querySelector<HTMLAnchorElement>('.pure-menu-link');
      link?.click();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    afterEach(() => {
      document.querySelectorAll('toast-message').forEach(el => el.remove());
    });

    it('should pass the exact identifier (including special characters) to the print API', () => {
      expect(fetchStub.calledTwice).to.be.true;
      const requestBody = JSON.parse((fetchStub.secondCall.args[1] as RequestInit).body as string) as Record<string, unknown>;
      expect(requestBody['template_identifier']).to.equal(specialIdentifier);
    });
  });

  describe('when clicking a menu item', () => {
    beforeEach(async () => {
      // First call: find_by_key_existence returns one item
      fetchStub.onFirstCall().resolves(makeJsonResponse({
        ids: [{ identifier: 'zebra_4x2', title: 'Zebra 4x2' }],
      }));
      // Second call (from printLabel): return success
      fetchStub.onSecondCall().resolves(makeJsonResponse({ success: true, message: 'Printed' }));
      initPrintMenu();
      await new Promise(resolve => setTimeout(resolve, 50));

      const link = document.querySelector<HTMLAnchorElement>('.pure-menu-link');
      link?.click();
      await new Promise(resolve => setTimeout(resolve, 50));
    });

    afterEach(() => {
      document.querySelectorAll('toast-message').forEach(el => el.remove());
    });

    it('should call /api/print_label when link is clicked', () => {
      expect(fetchStub.calledTwice).to.be.true;
      expect(fetchStub.secondCall.args[0]).to.equal('/api/print_label');
    });
  });
});
