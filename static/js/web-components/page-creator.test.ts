import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import { PageCreator } from './page-creator.js';

describe('PageCreator', () => {
  let service: PageCreator;

  beforeEach(() => {
    service = new PageCreator();
  });

  afterEach(() => {
    sinon.restore();
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('listTemplates', () => {
    describe('when client returns templates', () => {
      let result: Awaited<ReturnType<typeof service.listTemplates>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { listTemplates: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'listTemplates').resolves({
          templates: [
            { identifier: 'article_template', title: 'Article', description: 'Standard article format' },
            { identifier: 'project_template', title: 'Project', description: 'Project documentation' },
          ],
        });

        result = await service.listTemplates();
      });

      it('should return templates array', () => {
        expect(result.templates).to.have.length(2);
      });

      it('should return first template identifier', () => {
        expect(result.templates[0]!.identifier).to.equal('article_template');
      });

      it('should return first template title', () => {
        expect(result.templates[0]!.title).to.equal('Article');
      });

      it('should return first template description', () => {
        expect(result.templates[0]!.description).to.equal('Standard article format');
      });

      it('should not have error', () => {
        expect(result.error).to.be.undefined;
      });

      it('should call client with empty excludeIdentifiers', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.excludeIdentifiers).to.deep.equal([]);
      });
    });

    describe('when called with excludeIdentifiers', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { listTemplates: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'listTemplates').resolves({
          templates: [{ identifier: 'article_template', title: 'Article', description: '' }],
        });

        await service.listTemplates(['inv_item', 'system_template']);
      });

      it('should pass excludeIdentifiers to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.excludeIdentifiers).to.deep.equal(['inv_item', 'system_template']);
      });
    });

    describe('when client returns empty templates', () => {
      let result: Awaited<ReturnType<typeof service.listTemplates>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { listTemplates: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'listTemplates').resolves({
          templates: [],
        });

        result = await service.listTemplates();
      });

      it('should return empty templates array', () => {
        expect(result.templates).to.have.length(0);
      });

      it('should not have error', () => {
        expect(result.error).to.be.undefined;
      });
    });

    describe('when client throws error', () => {
      let result: Awaited<ReturnType<typeof service.listTemplates>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { listTemplates: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'listTemplates').rejects(new Error('Network error'));

        result = await service.listTemplates();
      });

      it('should return empty templates array', () => {
        expect(result.templates).to.have.length(0);
      });

      it('should return error', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('getTemplateFrontmatter', () => {
    describe('when client returns frontmatter', () => {
      let result: Awaited<ReturnType<typeof service.getTemplateFrontmatter>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { frontmatterClient: { getFrontmatter: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.frontmatterClient, 'getFrontmatter').resolves({
          frontmatter: {
            template: 'article_template',
            author: 'Default Author',
            category: 'articles',
            tags: ['draft'],
          },
        });

        result = await service.getTemplateFrontmatter('article_template');
      });

      it('should return frontmatter object', () => {
        expect(result.frontmatter).to.deep.equal({
          template: 'article_template',
          author: 'Default Author',
          category: 'articles',
          tags: ['draft'],
        });
      });

      it('should not have error', () => {
        expect(result.error).to.be.undefined;
      });

      it('should call client with correct page identifier', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.page).to.equal('article_template');
      });
    });

    describe('when client returns null frontmatter', () => {
      let result: Awaited<ReturnType<typeof service.getTemplateFrontmatter>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { frontmatterClient: { getFrontmatter: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.frontmatterClient, 'getFrontmatter').resolves({
          frontmatter: null,
        });

        result = await service.getTemplateFrontmatter('empty_template');
      });

      it('should return empty object', () => {
        expect(result.frontmatter).to.deep.equal({});
      });

      it('should not have error', () => {
        expect(result.error).to.be.undefined;
      });
    });

    describe('when client throws error', () => {
      let result: Awaited<ReturnType<typeof service.getTemplateFrontmatter>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { frontmatterClient: { getFrontmatter: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.frontmatterClient, 'getFrontmatter').rejects(new Error('Network error'));

        result = await service.getTemplateFrontmatter('article_template');
      });

      it('should return empty object', () => {
        expect(result.frontmatter).to.deep.equal({});
      });

      it('should return error', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('generateIdentifier', () => {
    describe('when called with empty text', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        result = await service.generateIdentifier('');
      });

      it('should return empty identifier', () => {
        expect(result.identifier).to.equal('');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });
    });

    describe('when called with valid text and identifier is unique', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'my_new_article',
          isUnique: true,
          existingPage: undefined,
        });

        result = await service.generateIdentifier('My New Article');
      });

      it('should return the generated identifier', () => {
        expect(result.identifier).to.equal('my_new_article');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });

      it('should not have existingPage', () => {
        expect(result.existingPage).to.be.undefined;
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.text).to.equal('My New Article');
        expect(request.ensureUnique).to.be.false;
      });
    });

    describe('when called with valid text and identifier already exists', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'existing_page',
          isUnique: false,
          existingPage: {
            identifier: 'existing_page',
            title: 'Existing Page',
          },
        });

        result = await service.generateIdentifier('Existing Page');
      });

      it('should return the generated identifier', () => {
        expect(result.identifier).to.equal('existing_page');
      });

      it('should return isUnique false', () => {
        expect(result.isUnique).to.be.false;
      });

      it('should return existingPage with identifier', () => {
        expect(result.existingPage?.identifier).to.equal('existing_page');
      });

      it('should return existingPage with title', () => {
        expect(result.existingPage?.title).to.equal('Existing Page');
      });
    });

    describe('when called with ensureUnique true', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').resolves({
          identifier: 'my_page_1',
          isUnique: true,
          existingPage: undefined,
        });

        await service.generateIdentifier('My Page', true);
      });

      it('should call client with ensureUnique true', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.ensureUnique).to.be.true;
      });
    });

    describe('when client throws error', () => {
      let result: Awaited<ReturnType<typeof service.generateIdentifier>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { generateIdentifier: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'generateIdentifier').rejects(new Error('Network error'));

        result = await service.generateIdentifier('Some Text');
      });

      it('should return empty identifier', () => {
        expect(result.identifier).to.equal('');
      });

      it('should return isUnique true', () => {
        expect(result.isUnique).to.be.true;
      });

      it('should return error', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('createPage', () => {
    describe('when called with empty identifier', () => {
      let result: Awaited<ReturnType<typeof service.createPage>>;

      beforeEach(async () => {
        result = await service.createPage('');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return validation error as Error object', () => {
        expect(result.error).to.be.instanceOf(Error);
        expect(result.error?.message).to.equal('Identifier is required');
      });
    });

    describe('when called with valid identifier and client returns success', () => {
      let result: Awaited<ReturnType<typeof service.createPage>>;
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: true,
        });

        result = await service.createPage('my_new_page');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should not have error', () => {
        expect(result.error).to.be.undefined;
      });

      it('should call client with correct request', () => {
        expect(clientStub).to.have.been.calledOnce;
        const request = clientStub.firstCall.args[0];
        expect(request.pageName).to.equal('my_new_page');
        expect(request.contentMarkdown).to.equal('');
        expect(request.frontmatter).to.be.undefined;
      });
    });

    describe('when called with contentMarkdown', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: true,
        });

        await service.createPage('my_page', '# My Page\n\nSome content here.');
      });

      it('should pass contentMarkdown to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.contentMarkdown).to.equal('# My Page\n\nSome content here.');
      });
    });

    describe('when called with template', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: true,
        });

        await service.createPage('my_article', '', 'article_template');
      });

      it('should pass template to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.template).to.equal('article_template');
      });
    });

    describe('when called with frontmatter', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: true,
        });

        await service.createPage('my_page', '', undefined, { author: 'John', tags: ['blog'] });
      });

      it('should pass frontmatter to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.frontmatter).to.deep.equal({ author: 'John', tags: ['blog'] });
      });
    });

    describe('when called with all parameters', () => {
      let clientStub: sinon.SinonStub;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        clientStub = sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: true,
        });

        await service.createPage('my_article', '# Content', 'article_template', { author: 'John' });
      });

      it('should pass all parameters to client', () => {
        const request = clientStub.firstCall.args[0];
        expect(request.pageName).to.equal('my_article');
        expect(request.contentMarkdown).to.equal('# Content');
        expect(request.template).to.equal('article_template');
        expect(request.frontmatter).to.deep.equal({ author: 'John' });
      });
    });

    describe('when client returns error response', () => {
      let result: Awaited<ReturnType<typeof service.createPage>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: false,
          error: 'Page already exists',
        });

        result = await service.createPage('existing_page');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return the error as Error object', () => {
        expect(result.error).to.be.instanceOf(Error);
        expect(result.error?.message).to.equal('Page already exists');
      });
    });

    describe('when client returns failure without error message', () => {
      let result: Awaited<ReturnType<typeof service.createPage>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'createPage').resolves({
          success: false,
          error: '',
        });

        result = await service.createPage('some_page');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return default error message', () => {
        expect(result.error).to.be.instanceOf(Error);
        expect(result.error?.message).to.equal('Failed to create page');
      });
    });

    describe('when client throws error', () => {
      let result: Awaited<ReturnType<typeof service.createPage>>;

      beforeEach(async () => {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private client for testing
        const serviceWithClient = service as unknown as { pageManagementClient: { createPage: () => Promise<unknown> } };
        sinon.stub(serviceWithClient.pageManagementClient, 'createPage').rejects(new Error('Network error'));

        result = await service.createPage('my_page');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should return error', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('showSuccess', () => {
    // Note: showSuccess delegates to showToast/showToastAfter from toast-message.js
    // ES Modules cannot be stubbed directly with sinon, so we test the interface contract

    describe('when called with message only', () => {
      it('should not throw', () => {
        expect(() => service.showSuccess('Page created')).to.not.throw();
      });
    });

    describe('when called with message and callback', () => {
      it('should not throw', () => {
        const callback = sinon.stub();
        expect(() => service.showSuccess('Page created', callback)).to.not.throw();
      });
    });
  });

  describe('showError', () => {
    // Note: showError delegates to showToast from toast-message.js
    // ES Modules cannot be stubbed directly with sinon, so we test the interface contract

    describe('when called with message', () => {
      it('should not throw', () => {
        expect(() => service.showError('Something went wrong')).to.not.throw();
      });
    });
  });
});
