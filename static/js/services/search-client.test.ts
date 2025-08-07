import { expect } from '@open-wc/testing';
import { stub, SinonStub } from 'sinon';
import { SearchClient } from './search-client.js';
import { SearchContentResponse, SearchResult, HighlightSpan } from '../gen/api/v1/search_pb.js';

describe('SearchClient', () => {
  let searchClient: SearchClient;
  let searchContentStub: SinonStub;

  beforeEach(() => {
    searchClient = new SearchClient();
    // Stub the gRPC client method
    searchContentStub = stub((searchClient as { client: object }).client, 'searchContent');
  });

  afterEach(() => {
    searchContentStub.restore();
  });

  describe('search', () => {
    describe('when searching with a valid query', () => {
      let results: SearchResult[];
      const mockResponse = new SearchContentResponse({
        results: [
          new SearchResult({
            identifier: 'test-page',
            title: 'Test Page',
            fragment: 'This is a test fragment with highlighted text.',
            highlights: [
              new HighlightSpan({ start: 10, end: 14 }), // "test"
              new HighlightSpan({ start: 29, end: 40 }), // "highlighted"
            ],
          }),
          new SearchResult({
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
        expect(results[0].identifier).to.equal('test-page');
        expect(results[0].title).to.equal('Test Page');
        expect(results[0].fragment).to.equal('This is a test fragment with highlighted text.');
        expect(results[0].highlights).to.have.lengthOf(2);
        expect(results[1].identifier).to.equal('another-page');
        expect(results[1].title).to.equal('Another Page');
        expect(results[1].fragment).to.equal('No highlights here.');
        expect(results[1].highlights).to.have.lengthOf(0);
      });
    });

    describe('when searching returns no results', () => {
      let results: SearchResult[];

      beforeEach(async () => {
        searchContentStub.resolves(new SearchContentResponse({ results: [] }));
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
          error = e as Error;
        }
      });

      it('should propagate the error', () => {
        expect(error).to.exist;
        expect(error.message).to.equal('Network error');
      });
    });
  });

  describe('simplified data structure', () => {
    it('should return structured data without HTML generation', () => {
      const mockResponse = new SearchContentResponse({
        results: [
          new SearchResult({
            identifier: 'structured-data-test',
            title: 'Structured Data Test',
            fragment: 'This fragment contains <script>dangerous</script> content & symbols.',
            highlights: [
              new HighlightSpan({ start: 21, end: 29 }), // "<script>"
              new HighlightSpan({ start: 54, end: 61 }), // "content"
            ],
          }),
        ],
      });

      searchContentStub.resolves(mockResponse);
      
      return searchClient.search('test').then(results => {
        // Should return the raw data without HTML processing
        expect(results[0].fragment).to.equal('This fragment contains <script>dangerous</script> content & symbols.');
        expect(results[0].highlights).to.have.lengthOf(2);
        expect(results[0].highlights[0].start).to.equal(21);
        expect(results[0].highlights[0].end).to.equal(29);
        expect(results[0].highlights[1].start).to.equal(54);
        expect(results[0].highlights[1].end).to.equal(61);
        // Should NOT have fragmentHTML property
        expect((results[0] as any).fragmentHTML).to.be.undefined;
      });
    });
  });
});