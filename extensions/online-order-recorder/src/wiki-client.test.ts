import { describe, it, expect, beforeEach, vi } from 'vitest';

const mockReadPage = vi.fn();
const mockUpdatePageContent = vi.fn();
const mockCreatePage = vi.fn();

const mockClient = {
  readPage: mockReadPage,
  updatePageContent: mockUpdatePageContent,
  createPage: mockCreatePage,
};

const mockCreateClient = vi.fn(() => mockClient);
const mockTransport = { _mock: true };
const mockCreateGrpcWebTransport = vi.fn(() => mockTransport);
const mockCreate = vi.fn((_schema: unknown, data: unknown) => data);

vi.mock('@connectrpc/connect', () => ({
  createClient: mockCreateClient,
}));

vi.mock('@connectrpc/connect-web', () => ({
  createGrpcWebTransport: mockCreateGrpcWebTransport,
}));

vi.mock('@bufbuild/protobuf', () => ({
  create: mockCreate,
}));

vi.mock('../../../static/js/gen/api/v1/page_management_pb.js', () => ({
  PageManagementService: { _service: 'PageManagementService' },
  ReadPageRequestSchema: { _schema: 'ReadPageRequest' },
  CreatePageRequestSchema: { _schema: 'CreatePageRequest' },
  UpdatePageContentRequestSchema: { _schema: 'UpdatePageContentRequest' },
}));

describe('readPage', () => {
  let readPageFn: typeof import('./wiki-client.js').readPage;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();

    // Re-apply mocks after resetModules
    vi.doMock('@connectrpc/connect', () => ({
      createClient: mockCreateClient,
    }));
    vi.doMock('@connectrpc/connect-web', () => ({
      createGrpcWebTransport: mockCreateGrpcWebTransport,
    }));
    vi.doMock('@bufbuild/protobuf', () => ({
      create: mockCreate,
    }));
    vi.doMock('../../../static/js/gen/api/v1/page_management_pb.js', () => ({
      PageManagementService: { _service: 'PageManagementService' },
      ReadPageRequestSchema: { _schema: 'ReadPageRequest' },
      CreatePageRequestSchema: { _schema: 'CreatePageRequest' },
      UpdatePageContentRequestSchema: { _schema: 'UpdatePageContentRequest' },
    }));

    const mod = await import('./wiki-client.js');
    readPageFn = mod.readPage;
  });

  describe('when page exists', () => {
    let result: { contentMarkdown: string; versionHash: string };

    beforeEach(async () => {
      mockReadPage.mockResolvedValue({
        contentMarkdown: '# Hello',
        versionHash: 'abc123',
      });

      result = await readPageFn('https://wiki.example.com', 'test_page');
    });

    it('should create transport with trimmed base URL', () => {
      expect(mockCreateGrpcWebTransport).toHaveBeenCalledWith({
        baseUrl: 'https://wiki.example.com',
      });
    });

    it('should create client with PageManagementService', () => {
      expect(mockCreateClient).toHaveBeenCalledWith(
        { _service: 'PageManagementService' },
        mockTransport
      );
    });

    it('should create request with pageName', () => {
      expect(mockCreate).toHaveBeenCalledWith(
        { _schema: 'ReadPageRequest' },
        { pageName: 'test_page' }
      );
    });

    it('should return contentMarkdown', () => {
      expect(result.contentMarkdown).to.equal('# Hello');
    });

    it('should return versionHash', () => {
      expect(result.versionHash).to.equal('abc123');
    });
  });

  describe('when URL has trailing slash', () => {
    beforeEach(async () => {
      mockReadPage.mockResolvedValue({ contentMarkdown: '', versionHash: '' });
      await readPageFn('https://wiki.example.com/', 'test_page');
    });

    it('should strip trailing slash from base URL', () => {
      expect(mockCreateGrpcWebTransport).toHaveBeenCalledWith({
        baseUrl: 'https://wiki.example.com',
      });
    });
  });

  describe('when client.readPage rejects', () => {
    let thrownError: Error;

    beforeEach(async () => {
      mockReadPage.mockRejectedValue(new Error('network failure'));
      try {
        await readPageFn('https://wiki.example.com', 'test_page');
      } catch (err) {
        thrownError = err as Error;
      }
    });

    it('should propagate the error', () => {
      expect(thrownError).toBeInstanceOf(Error);
      expect(thrownError.message).to.equal('network failure');
    });
  });
});

describe('updatePageContent', () => {
  let updatePageContentFn: typeof import('./wiki-client.js').updatePageContent;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();

    vi.doMock('@connectrpc/connect', () => ({
      createClient: mockCreateClient,
    }));
    vi.doMock('@connectrpc/connect-web', () => ({
      createGrpcWebTransport: mockCreateGrpcWebTransport,
    }));
    vi.doMock('@bufbuild/protobuf', () => ({
      create: mockCreate,
    }));
    vi.doMock('../../../static/js/gen/api/v1/page_management_pb.js', () => ({
      PageManagementService: { _service: 'PageManagementService' },
      ReadPageRequestSchema: { _schema: 'ReadPageRequest' },
      CreatePageRequestSchema: { _schema: 'CreatePageRequest' },
      UpdatePageContentRequestSchema: { _schema: 'UpdatePageContentRequest' },
    }));

    const mod = await import('./wiki-client.js');
    updatePageContentFn = mod.updatePageContent;
  });

  describe('when update succeeds', () => {
    beforeEach(async () => {
      mockUpdatePageContent.mockResolvedValue({});
      await updatePageContentFn(
        'https://wiki.example.com',
        'test_page',
        '# Updated',
        'hash456'
      );
    });

    it('should create request with all fields', () => {
      expect(mockCreate).toHaveBeenCalledWith(
        { _schema: 'UpdatePageContentRequest' },
        {
          pageName: 'test_page',
          newContentMarkdown: '# Updated',
          expectedVersionHash: 'hash456',
        }
      );
    });

    it('should call client.updatePageContent', () => {
      expect(mockUpdatePageContent).toHaveBeenCalledOnce();
    });
  });

  describe('when update fails with version mismatch', () => {
    let thrownError: Error;

    beforeEach(async () => {
      mockUpdatePageContent.mockRejectedValue(new Error('version mismatch'));
      try {
        await updatePageContentFn('https://wiki.example.com', 'p', 'md', 'h');
      } catch (err) {
        thrownError = err as Error;
      }
    });

    it('should propagate the error', () => {
      expect(thrownError.message).to.equal('version mismatch');
    });
  });
});

describe('createPage', () => {
  let createPageFn: typeof import('./wiki-client.js').createPage;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();

    vi.doMock('@connectrpc/connect', () => ({
      createClient: mockCreateClient,
    }));
    vi.doMock('@connectrpc/connect-web', () => ({
      createGrpcWebTransport: mockCreateGrpcWebTransport,
    }));
    vi.doMock('@bufbuild/protobuf', () => ({
      create: mockCreate,
    }));
    vi.doMock('../../../static/js/gen/api/v1/page_management_pb.js', () => ({
      PageManagementService: { _service: 'PageManagementService' },
      ReadPageRequestSchema: { _schema: 'ReadPageRequest' },
      CreatePageRequestSchema: { _schema: 'CreatePageRequest' },
      UpdatePageContentRequestSchema: { _schema: 'UpdatePageContentRequest' },
    }));

    const mod = await import('./wiki-client.js');
    createPageFn = mod.createPage;
  });

  describe('when creation succeeds', () => {
    beforeEach(async () => {
      mockCreatePage.mockResolvedValue({});
      await createPageFn('https://wiki.example.com', 'new_page', '# New');
    });

    it('should create request with pageName and contentMarkdown', () => {
      expect(mockCreate).toHaveBeenCalledWith(
        { _schema: 'CreatePageRequest' },
        {
          pageName: 'new_page',
          contentMarkdown: '# New',
        }
      );
    });

    it('should call client.createPage', () => {
      expect(mockCreatePage).toHaveBeenCalledOnce();
    });
  });

  describe('when creation fails', () => {
    let thrownError: Error;

    beforeEach(async () => {
      mockCreatePage.mockRejectedValue(new Error('page already exists'));
      try {
        await createPageFn('https://wiki.example.com', 'p', 'md');
      } catch (err) {
        thrownError = err as Error;
      }
    });

    it('should propagate the error', () => {
      expect(thrownError.message).to.equal('page already exists');
    });
  });
});

describe('client caching', () => {
  let readPageFn: typeof import('./wiki-client.js').readPage;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();

    vi.doMock('@connectrpc/connect', () => ({
      createClient: mockCreateClient,
    }));
    vi.doMock('@connectrpc/connect-web', () => ({
      createGrpcWebTransport: mockCreateGrpcWebTransport,
    }));
    vi.doMock('@bufbuild/protobuf', () => ({
      create: mockCreate,
    }));
    vi.doMock('../../../static/js/gen/api/v1/page_management_pb.js', () => ({
      PageManagementService: { _service: 'PageManagementService' },
      ReadPageRequestSchema: { _schema: 'ReadPageRequest' },
      CreatePageRequestSchema: { _schema: 'CreatePageRequest' },
      UpdatePageContentRequestSchema: { _schema: 'UpdatePageContentRequest' },
    }));

    const mod = await import('./wiki-client.js');
    readPageFn = mod.readPage;
    mockReadPage.mockResolvedValue({ contentMarkdown: '', versionHash: '' });
  });

  describe('when called twice with the same URL', () => {
    beforeEach(async () => {
      await readPageFn('https://wiki.example.com', 'page1');
      await readPageFn('https://wiki.example.com', 'page2');
    });

    it('should create transport only once', () => {
      expect(mockCreateGrpcWebTransport).toHaveBeenCalledOnce();
    });

    it('should create client only once', () => {
      expect(mockCreateClient).toHaveBeenCalledOnce();
    });
  });

  describe('when called with different URLs', () => {
    beforeEach(async () => {
      await readPageFn('https://wiki-a.example.com', 'page1');
      await readPageFn('https://wiki-b.example.com', 'page2');
    });

    it('should create transport twice', () => {
      expect(mockCreateGrpcWebTransport).toHaveBeenCalledTimes(2);
    });

    it('should create client twice', () => {
      expect(mockCreateClient).toHaveBeenCalledTimes(2);
    });
  });
});
