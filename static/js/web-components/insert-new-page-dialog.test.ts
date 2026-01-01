import { expect, fixture, html, waitUntil } from '@open-wc/testing';
import sinon from 'sinon';
import './insert-new-page-dialog.js';
import type { InsertNewPageDialog } from './insert-new-page-dialog.js';
import type { PageCreator } from './page-creator.js';

describe('InsertNewPageDialog', () => {
  let el: InsertNewPageDialog;

  afterEach(() => {
    sinon.restore();
  });

  describe('when created', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);
    });

    it('should exist', () => {
      expect(el).to.exist;
    });

    it('should not be open initially', () => {
      expect(el.open).to.be.false;
    });

    it('should not display dialog when not open', () => {
      expect(el.shadowRoot?.querySelector('.dialog')).to.exist;
      // :host display: none is handled by CSS
    });
  });

  describe('openDialog', () => {
    let listTemplatesStub: sinon.SinonStub;

    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // Stub the pageCreator methods
      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      listTemplatesStub = sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({
        templates: [
          { identifier: 'article_template', title: 'Article', description: '' },
          { identifier: 'project_template', title: 'Project', description: '' },
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
        ] as any,
      });

      await el.openDialog();
      await el.updateComplete;
    });

    it('should set open to true', () => {
      expect(el.open).to.be.true;
    });

    it('should load templates', () => {
      expect(listTemplatesStub).to.have.been.calledOnce;
    });

    it('should exclude inv_item template', () => {
      const args = listTemplatesStub.firstCall.args[0];
      expect(args).to.deep.equal(['inv_item']);
    });

    it('should render dialog title', () => {
      const title = el.shadowRoot?.querySelector('.dialog-title');
      expect(title?.textContent).to.equal('Insert New Page');
    });

    it('should render automagic identifier input', () => {
      const input = el.shadowRoot?.querySelector('automagic-identifier-input');
      expect(input).to.exist;
    });

    it('should render template selector', () => {
      const select = el.shadowRoot?.querySelector('select[name="template"]');
      expect(select).to.exist;
    });

    it('should populate template options', () => {
      const options = el.shadowRoot?.querySelectorAll('select[name="template"] option');
      expect(options?.length).to.equal(3); // (none) + 2 templates
    });

    it('should render frontmatter section', () => {
      const section = el.shadowRoot?.querySelector('.frontmatter-section');
      expect(section).to.exist;
    });

    it('should render cancel button', () => {
      const button = el.shadowRoot?.querySelector('.button-secondary');
      expect(button?.textContent?.trim()).to.equal('Cancel');
    });

    it('should render create button', () => {
      const button = el.shadowRoot?.querySelector('.button-primary');
      expect(button?.textContent?.trim()).to.equal('Create Page');
    });
  });

  describe('close', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });

      await el.openDialog();
      await el.updateComplete;
      el.close();
      await el.updateComplete;
    });

    it('should set open to false', () => {
      expect(el.open).to.be.false;
    });

    it('should reset pageTitle', () => {
      expect(el.pageTitle).to.equal('');
    });

    it('should reset pageIdentifier', () => {
      expect(el.pageIdentifier).to.equal('');
    });

    it('should reset selectedTemplate', () => {
      expect(el.selectedTemplate).to.equal('');
    });

    it('should reset templateLocked', () => {
      expect(el.templateLocked).to.be.false;
    });
  });

  describe('when clicking backdrop', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });

      await el.openDialog();
      await el.updateComplete;

      const backdrop = el.shadowRoot?.querySelector('.backdrop') as HTMLElement;
      backdrop.click();
      await el.updateComplete;
    });

    it('should close dialog', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('when pressing escape key', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });

      await el.openDialog();
      await el.updateComplete;

      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
      await el.updateComplete;
    });

    it('should close dialog', () => {
      expect(el.open).to.be.false;
    });
  });

  describe('template selection', () => {
    let getTemplateFrontmatterStub: sinon.SinonStub;

    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        templates: [{ identifier: 'article_template', title: 'Article', description: '' }] as any,
      });
      getTemplateFrontmatterStub = sinon.stub(serviceWithCreator.pageCreator, 'getTemplateFrontmatter').resolves({
        frontmatter: {
          template: 'article_template',
          identifier: 'article_template',
          author: 'Default Author',
          category: 'articles',
          tags: ['draft'],
        },
      });

      await el.openDialog();
      await el.updateComplete;
    });

    describe('when template is selected', () => {
      beforeEach(async () => {
        const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
        select.value = 'article_template';
        select.dispatchEvent(new Event('change'));
        // Wait for async template frontmatter fetch to complete
        await waitUntil(() => el.templateLocked, 'templateLocked should become true');
        await el.updateComplete;
      });

      it('should set selectedTemplate', () => {
        expect(el.selectedTemplate).to.equal('article_template');
      });

      it('should lock template dropdown', () => {
        expect(el.templateLocked).to.be.true;
      });

      it('should disable template dropdown', () => {
        const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
        expect(select.disabled).to.be.true;
      });

      it('should show change button', () => {
        const changeButton = el.shadowRoot?.querySelector('confirmation-interlock-button');
        expect(changeButton).to.exist;
      });

      it('should call getTemplateFrontmatter', () => {
        expect(getTemplateFrontmatterStub).to.have.been.calledOnceWith('article_template');
      });

      it('should populate frontmatter with template data', () => {
        expect(el.frontmatter).to.deep.equal({
          author: 'Default Author',
          category: 'articles',
          tags: ['draft'],
        });
      });

      it('should filter out template key from frontmatter', () => {
        expect(el.frontmatter).to.not.have.property('template');
      });

      it('should filter out identifier key from frontmatter', () => {
        expect(el.frontmatter).to.not.have.property('identifier');
      });
    });

    describe('when template frontmatter fetch fails', () => {
      beforeEach(async () => {
        getTemplateFrontmatterStub.resolves({
          frontmatter: {},
          error: new Error('Failed to fetch'),
        });

        const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
        select.value = 'article_template';
        select.dispatchEvent(new Event('change'));
        // Wait for error to be set (indicates fetch completed)
        await waitUntil(() => el.error !== null, 'error should be set');
        await el.updateComplete;
      });

      it('should set frontmatter to empty object', () => {
        expect(el.frontmatter).to.deep.equal({});
      });

      it('should set error for display', () => {
        expect(el.error).to.exist;
      });

      it('should unlock template dropdown so user can try different template', () => {
        expect(el.templateLocked).to.be.false;
      });

      it('should clear selected template', () => {
        expect(el.selectedTemplate).to.equal('');
      });
    });
  });

  describe('template change confirmation', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        templates: [{ identifier: 'article_template', title: 'Article', description: '' }] as any,
      });
      sinon.stub(serviceWithCreator.pageCreator, 'getTemplateFrontmatter').resolves({
        frontmatter: { author: 'Test Author' },
      });

      await el.openDialog();
      await el.updateComplete;

      // Select a template
      const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
      select.value = 'article_template';
      select.dispatchEvent(new Event('change'));
      // Wait for async template frontmatter fetch to complete
      await waitUntil(() => el.templateLocked, 'templateLocked should become true');
      await el.updateComplete;
    });

    describe('when confirmed', () => {
      beforeEach(async () => {
        const interlockButton = el.shadowRoot?.querySelector('confirmation-interlock-button');
        interlockButton?.dispatchEvent(new CustomEvent('confirmed'));
        await el.updateComplete;
      });

      it('should unlock template dropdown', () => {
        expect(el.templateLocked).to.be.false;
      });

      it('should clear selected template', () => {
        expect(el.selectedTemplate).to.equal('');
      });

      it('should clear frontmatter', () => {
        expect(el.frontmatter).to.deep.equal({});
      });

      it('should enable template dropdown', () => {
        const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
        expect(select.disabled).to.be.false;
      });
    });
  });

  describe('submit button state', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });

      await el.openDialog();
      await el.updateComplete;
    });

    describe('when identifier is empty', () => {
      it('should disable create button', () => {
        const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(button.disabled).to.be.true;
      });
    });

    describe('when identifier is valid and unique', () => {
      beforeEach(async () => {
        el.pageIdentifier = 'my_new_page';
        el.isUnique = true;
        await el.updateComplete;
      });

      it('should enable create button', () => {
        const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(button.disabled).to.be.false;
      });
    });

    describe('when identifier exists', () => {
      beforeEach(async () => {
        el.pageIdentifier = 'existing_page';
        el.isUnique = false;
        await el.updateComplete;
      });

      it('should disable create button', () => {
        const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        expect(button.disabled).to.be.true;
      });
    });
  });

  describe('page creation', () => {
    let createPageStub: sinon.SinonStub;
    let showSuccessStub: sinon.SinonStub;
    let pageCreatedHandler: sinon.SinonStub;

    beforeEach(async () => {
      pageCreatedHandler = sinon.stub();
      el = await fixture(html`
        <insert-new-page-dialog @page-created=${pageCreatedHandler}></insert-new-page-dialog>
      `);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });
      sinon.stub(serviceWithCreator.pageCreator, 'generateIdentifier').resolves({
        identifier: 'my_article',
        isUnique: true,
      });
      createPageStub = sinon.stub(serviceWithCreator.pageCreator, 'createPage').resolves({
        success: true,
      });
      showSuccessStub = sinon.stub(serviceWithCreator.pageCreator, 'showSuccess');

      await el.openDialog();
      await el.updateComplete;

      // Set up valid state
      el.pageTitle = 'My Article';
      el.pageIdentifier = 'my_article';
      el.isUnique = true;
      await el.updateComplete;
    });

    describe('when create succeeds', () => {
      beforeEach(async () => {
        const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        button.click();
        await el.updateComplete;
        await waitUntil(() => !el.loading);
      });

      it('should call createPage with identifier', () => {
        expect(createPageStub).to.have.been.calledOnce;
        const args = createPageStub.firstCall.args;
        expect(args[0]).to.equal('my_article');
      });

      it('should dispatch page-created event', () => {
        expect(pageCreatedHandler).to.have.been.calledOnce;
      });

      it('should include identifier in event detail', () => {
        const detail = pageCreatedHandler.firstCall.args[0].detail;
        expect(detail.identifier).to.equal('my_article');
      });

      it('should include title in event detail', () => {
        const detail = pageCreatedHandler.firstCall.args[0].detail;
        expect(detail.title).to.equal('My Article');
      });

      it('should include markdownLink in event detail', () => {
        const detail = pageCreatedHandler.firstCall.args[0].detail;
        expect(detail.markdownLink).to.equal('[My Article](/my_article)');
      });

      it('should show success toast', () => {
        expect(showSuccessStub).to.have.been.calledOnce;
      });

      it('should close dialog', () => {
        expect(el.open).to.be.false;
      });
    });

    describe('when create fails', () => {
      beforeEach(async () => {
        createPageStub.resolves({
          success: false,
          error: new Error('Page already exists'),
        });

        const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
        button.click();
        await el.updateComplete;
        await waitUntil(() => !el.loading);
      });

      it('should not dispatch page-created event', () => {
        expect(pageCreatedHandler).to.not.have.been.called;
      });

      it('should set error state', () => {
        expect(el.error).to.exist;
      });

      it('should display error', () => {
        const errorDisplay = el.shadowRoot?.querySelector('error-display');
        expect(errorDisplay).to.exist;
      });

      it('should keep dialog open', () => {
        expect(el.open).to.be.true;
      });
    });
  });

  describe('when title is empty and identifier is provided', () => {
    let pageCreatedHandler: sinon.SinonStub;

    beforeEach(async () => {
      pageCreatedHandler = sinon.stub();
      el = await fixture(html`
        <insert-new-page-dialog @page-created=${pageCreatedHandler}></insert-new-page-dialog>
      `);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });
      sinon.stub(serviceWithCreator.pageCreator, 'createPage').resolves({ success: true });
      sinon.stub(serviceWithCreator.pageCreator, 'showSuccess');

      await el.openDialog();
      await el.updateComplete;

      // Set identifier without title
      el.pageTitle = '';
      el.pageIdentifier = 'my_page';
      el.isUnique = true;
      await el.updateComplete;

      const button = el.shadowRoot?.querySelector('.button-primary') as HTMLButtonElement;
      button.click();
      await el.updateComplete;
      await waitUntil(() => !el.loading);
    });

    it('should use identifier as title in markdown link', () => {
      const detail = pageCreatedHandler.firstCall.args[0].detail;
      expect(detail.markdownLink).to.equal('[my_page](/my_page)');
    });
  });

  describe('no templates available', () => {
    beforeEach(async () => {
      el = await fixture(html`<insert-new-page-dialog></insert-new-page-dialog>`);

      // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
      const serviceWithCreator = el as unknown as { pageCreator: PageCreator };
      sinon.stub(serviceWithCreator.pageCreator, 'listTemplates').resolves({ templates: [] });

      await el.openDialog();
      await el.updateComplete;
    });

    it('should show disabled dropdown with note', () => {
      const select = el.shadowRoot?.querySelector('select[name="template"]') as HTMLSelectElement;
      expect(select.disabled).to.be.true;
    });

    it('should show "no templates defined" in option', () => {
      const option = el.shadowRoot?.querySelector('select[name="template"] option') as HTMLOptionElement;
      expect(option?.textContent).to.include('no templates defined');
    });
  });
});
