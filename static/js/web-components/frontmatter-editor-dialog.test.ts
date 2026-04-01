import { html, fixture, expect } from '@open-wc/testing';
import { FrontmatterEditorDialog } from './frontmatter-editor-dialog.js';
import { AugmentedError, ErrorKind } from './augment-error-service.js';
import sinon from 'sinon';
import './frontmatter-editor-dialog.js';

describe('FrontmatterEditorDialog', () => {
  let el: FrontmatterEditorDialog;

  function timeout(ms: number, message: string): Promise<never> {
    return new Promise((_, reject) =>
      setTimeout(() => reject(new Error(message)), ms)
    );
  }

  beforeEach(async () => {
    // Use Promise.race to add explicit timeout for fixture creation
    el = await Promise.race([
      fixture<FrontmatterEditorDialog>(html`<frontmatter-editor-dialog></frontmatter-editor-dialog>`),
      timeout(5000, 'Component fixture timed out')
    ]);

    // Stub the loadFrontmatter method to prevent network calls
    sinon.stub(el, 'loadFrontmatter').resolves();

    await el.updateComplete;
  });

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
  });

  it('should exist', () => {
    expect(el).to.not.equal(null);
  });

  it('should be an instance of FrontmatterEditorDialog', () => {
    expect(el).to.be.instanceOf(FrontmatterEditorDialog);
  });

  it('should have the correct tag name', () => {
    expect(el.tagName.toLowerCase()).to.equal('frontmatter-editor-dialog');
  });

  describe('when component is initialized', () => {
    it('should not be open by default', () => {
      expect(el.open).to.equal(false);
    });

    it('should have empty page by default', () => {
      expect(el.page).to.equal('');
    });

    it('should not be loading by default', () => {
      expect(el.loading).to.equal(false);
    });

    it('should have no error by default', () => {
      expect(el.augmentedError).to.equal(undefined);
    });
  });

  describe('_renderContent', () => {
    describe('when loading', () => {
      beforeEach(async () => {
        el.loading = true;

        await Promise.race([
          el.updateComplete,
          timeout(5000, 'Component update timed out'),
        ]);
      });

      it('should render the loading indicator', () => {
        const loadingEl = el.shadowRoot?.querySelector('.loading');
        expect(loadingEl).to.not.equal(null);
      });

      it('should show loading text', () => {
        const loadingEl = el.shadowRoot?.querySelector('.loading');
        expect(loadingEl?.textContent?.trim()).to.include('Loading frontmatter...');
      });

      it('should not render error-display', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.equal(null);
      });

      it('should not render frontmatter editor', () => {
        const editor = el.shadowRoot?.querySelector('frontmatter-value-section');
        expect(editor).to.equal(null);
      });
    });

    describe('when augmentedError is set', () => {
      beforeEach(async () => {
        el.augmentedError = new AugmentedError(new Error('Test error'), ErrorKind.ERROR, 'error');

        await Promise.race([
          el.updateComplete,
          timeout(5000, 'Component update timed out'),
        ]);
      });

      it('should render error-display component', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.not.equal(null);
      });

      it('should not render loading indicator', () => {
        const loadingEl = el.shadowRoot?.querySelector('.loading');
        expect(loadingEl).to.equal(null);
      });

      it('should not render frontmatter editor', () => {
        const editor = el.shadowRoot?.querySelector('frontmatter-value-section');
        expect(editor).to.equal(null);
      });
    });

    describe('when not loading and no error', () => {
      beforeEach(async () => {
        el.loading = false;
        el.augmentedError = undefined;
        el.workingFrontmatter = { title: 'Test' };
        el.open = true;

        await Promise.race([
          el.updateComplete,
          timeout(5000, 'Component update timed out'),
        ]);
      });

      it('should render the frontmatter editor', () => {
        const editor = el.shadowRoot?.querySelector('frontmatter-value-section');
        expect(editor).to.not.equal(null);
      });

      it('should not render loading indicator', () => {
        const loadingEl = el.shadowRoot?.querySelector('.loading');
        expect(loadingEl).to.equal(null);
      });

      it('should not render error-display', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.equal(null);
      });
    });
  });
});