import { expect } from '@open-wc/testing';
import { stub, type SinonStub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import { SearchClient } from './search-client.js';
import {
  SearchContentResponseSchema,
  SearchResultSchema,
  HighlightSpanSchema,
} from '../gen/api/v1/search_pb.js';
import type { SearchResult } from '../gen/api/v1/search_pb.js';

// Type interface for accessing private members in tests
interface SearchClientPrivate {
  getClient: () => { searchContent: unknown };
  client: { searchContent: unknown };
}

describe('SearchClient', () => {
  let searchClient: SearchClient;
  let searchContentStub: SinonStub;

  beforeEach(() => {
    searchClient = new SearchClient();
    // Ensure the client is initialized before stubbing
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private method for testing
    (searchClient as unknown as SearchClientPrivate).getClient();
    // Stub the gRPC client method
    // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- accessing private property for testing
    searchContentStub = stub((searchClient as unknown as SearchClientPrivate).client, 'searchContent');
  });

  afterEach(() => {
    searchContentStub.restore();
  });

  describe('search', () => {
    describe('when searching with a valid query', () => {
      let results: SearchResult[];
      const mockResponse = create(SearchContentResponseSchema, {
        results: [
          create(SearchResultSchema, {
            identifier: 'test-page',
            title: 'Test Page',
            fragment: 'This is a test fragment with highlighted text.',
            highlights: [
              create(HighlightSpanSchema, { start: 10, end: 14 }), // "test"
              create(HighlightSpanSchema, { start: 29, end: 40 }), // "highlighted"
            ],
          }),
          create(SearchResultSchema, {
            identifier: 'another-page',
            title: 'Another Page',
            fragment: 'No highlights here.',
            highlights: [],
          }),
        ],
      });

      beforeEach(async () => {
        searchContentStub.resolves(mockResponse);
        results = await searchClient.search('test query');
      });

      it('should call searchContent with the query', () => {
        expect(searchContentStub).to.have.been.calledOnce;
        expect(searchContentStub).to.have.been.calledWith({ query: 'test query' });
      });

      it('should return the correct number of results', () => {
        expect(results).to.have.lengthOf(2);
      });

      it('should preserve identifier, title, fragment, and highlights', () => {
        const firstResult = results[0];
        const secondResult = results[1];
        expect(firstResult).to.exist;
        expect(secondResult).to.exist;
        expect(firstResult!.identifier).to.equal('test-page');
        expect(firstResult!.title).to.equal('Test Page');
        expect(firstResult!.fragment).to.equal('This is a test fragment with highlighted text.');
        expect(firstResult!.highlights).to.have.lengthOf(2);
        expect(secondResult!.identifier).to.equal('another-page');
        expect(secondResult!.title).to.equal('Another Page');
        expect(secondResult!.fragment).to.equal('No highlights here.');
        expect(secondResult!.highlights).to.have.lengthOf(0);
      });
    });

    describe('when searching returns no results', () => {
      let results: SearchResult[];

      beforeEach(async () => {
        searchContentStub.resolves(create(SearchContentResponseSchema, { results: [] }));
        results = await searchClient.search('empty query');
      });

      it('should return an empty array', () => {
        expect(results).to.be.an('array').that.is.empty;
      });
    });

    describe('when the search fails', () => {
      let error: Error;

      beforeEach(async () => {
        searchContentStub.rejects(new Error('Network error'));
        try {
          await searchClient.search('failing query');
        } catch (e) {
          if (e instanceof Error) {
            error = e;
          }
        }
      });

      it('should propagate the error', () => {
        expect(error).to.exist;
        expect(error.message).to.equal('Network error');
      });
    });
  });

  describe('simplified data structure', () => {
    describe('when returning structured data', () => {
      let results: SearchResult[];

      beforeEach(async () => {
        const mockResponse = create(SearchContentResponseSchema, {
          results: [
            create(SearchResultSchema, {
              identifier: 'structured-data-test',
              title: 'Structured Data Test',
              fragment: 'This fragment contains <script>dangerous</script> content & symbols.',
              highlights: [
                create(HighlightSpanSchema, { start: 21, end: 29 }), // "<script>"
                create(HighlightSpanSchema, { start: 54, end: 61 }), // "content"
              ],
            }),
          ],
        });

        searchContentStub.resolves(mockResponse);
        results = await searchClient.search('test');
      });

      it('should return the raw fragment without HTML processing', () => {
        const firstResult = results[0];
        expect(firstResult).to.exist;
        expect(firstResult!.fragment).to.equal('This fragment contains <script>dangerous</script> content & symbols.');
      });

      it('should return highlights with correct positions', () => {
        const firstResult = results[0];
        expect(firstResult).to.exist;
        expect(firstResult!.highlights).to.have.lengthOf(2);
        const firstHighlight = firstResult!.highlights[0];
        const secondHighlight = firstResult!.highlights[1];
        expect(firstHighlight).to.exist;
        expect(secondHighlight).to.exist;
        expect(firstHighlight!.start).to.equal(21);
        expect(firstHighlight!.end).to.equal(29);
        expect(secondHighlight!.start).to.equal(54);
        expect(secondHighlight!.end).to.equal(61);
      });

      it('should NOT have fragmentHTML property', () => {
        const firstResult = results[0];
        expect(firstResult).to.exist;
        expect('fragmentHTML' in firstResult!).to.be.false;
      });
    });
  });
});