import { expect } from '@esm-bundle/chai';
import { stub, type SinonStub } from 'sinon';
import { ChatMarkdownRenderer } from './chat-markdown-renderer.js';
import type { Client } from '@connectrpc/connect';
import type { PageManagementService } from '../gen/api/v1/page_management_pb.js';

type MockClient = {
  renderMarkdown: SinonStub;
};

describe('ChatMarkdownRenderer', () => {
  let renderer: ChatMarkdownRenderer;
  let mockClient: MockClient;

  beforeEach(() => {
    mockClient = {
      renderMarkdown: stub(),
    };
    renderer = new ChatMarkdownRenderer(
      mockClient as unknown as Client<typeof PageManagementService>,
    );
  });

  describe('renderMarkdown', () => {
    describe('when content is empty', () => {
      let result: string;

      beforeEach(async () => {
        result = await renderer.renderMarkdown('');
      });

      it('should return empty string', () => {
        expect(result).to.equal('');
      });

      it('should not call the RPC', () => {
        expect(mockClient.renderMarkdown.called).to.be.false;
      });
    });

    describe('when content is provided', () => {
      let result: string;

      beforeEach(async () => {
        mockClient.renderMarkdown.resolves({ renderedHtml: '<h1>Hello</h1>' });
        result = await renderer.renderMarkdown('# Hello');
      });

      it('should return rendered HTML', () => {
        expect(result).to.equal('<h1>Hello</h1>');
      });

      it('should call the RPC with the content', () => {
        expect(mockClient.renderMarkdown.calledOnce).to.be.true;
        const arg = mockClient.renderMarkdown.firstCall.args[0] as { content: string; page: string };
        expect(arg.content).to.equal('# Hello');
      });
    });

    describe('when content is called twice with the same input', () => {
      beforeEach(async () => {
        mockClient.renderMarkdown.resolves({ renderedHtml: '<h1>Hello</h1>' });
        await renderer.renderMarkdown('# Hello');
        await renderer.renderMarkdown('# Hello');
      });

      it('should call the RPC only once (cache hit)', () => {
        expect(mockClient.renderMarkdown.calledOnce).to.be.true;
      });
    });

    describe('when called with different content', () => {
      beforeEach(async () => {
        mockClient.renderMarkdown.resolves({ renderedHtml: '<h1>First</h1>' });
        await renderer.renderMarkdown('# First');
        mockClient.renderMarkdown.resolves({ renderedHtml: '<h1>Second</h1>' });
        await renderer.renderMarkdown('# Second');
      });

      it('should call the RPC twice', () => {
        expect(mockClient.renderMarkdown.calledTwice).to.be.true;
      });
    });

    describe('when a page context is provided', () => {
      beforeEach(async () => {
        mockClient.renderMarkdown.resolves({ renderedHtml: '<p>with context</p>' });
        await renderer.renderMarkdown('some text', 'my-page');
      });

      it('should pass the page to the RPC', () => {
        const arg = mockClient.renderMarkdown.firstCall.args[0] as { content: string; page: string };
        expect(arg.page).to.equal('my-page');
      });
    });

    describe('when the RPC fails', () => {
      let error: Error;

      beforeEach(async () => {
        mockClient.renderMarkdown.rejects(new Error('network failure'));
        try {
          await renderer.renderMarkdown('# Hello');
        } catch (err) {
          error = err as Error;
        }
      });

      it('should propagate the error', () => {
        expect(error).to.be.instanceOf(Error);
      });

      it('should include the error message', () => {
        expect(error.message).to.equal('network failure');
      });
    });
  });
});
