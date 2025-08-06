import { expect } from '@open-wc/testing';
import { stub, SinonStub } from 'sinon';
import { SearchClient, SearchResultWithHTML } from './search-client.js';
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
      let results: SearchResultWithHTML[];
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

      it('should preserve identifier and title', () => {
        expect(results[0].identifier).to.equal('test-page');
        expect(results[0].title).to.equal('Test Page');
        expect(results[1].identifier).to.equal('another-page');
        expect(results[1].title).to.equal('Another Page');
      });

      it('should generate HTML with highlights', () => {
        expect(results[0].fragmentHTML).to.equal(
          'This is a <mark>test</mark> fragment with <mark>highlighted</mark> text.'
        );
      });

      it('should handle fragments without highlights', () => {
        expect(results[1].fragmentHTML).to.equal('No highlights here.');
      });
    });

    describe('when searching returns no results', () => {
      let results: SearchResultWithHTML[];

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

  describe('HTML fragment generation', () => {
    describe('when building HTML fragments', () => {
      it('should escape HTML special characters', () => {
        const mockResponse = new SearchContentResponse({
          results: [
            new SearchResult({
              identifier: 'xss-test',
              title: 'XSS Test',
              fragment: '<script>alert("XSS")</script> & <b>bold</b>',
              highlights: [],
            }),
          ],
        });

        searchContentStub.resolves(mockResponse);
        
        return searchClient.search('xss').then(results => {
          expect(results[0].fragmentHTML).to.equal(
            '&lt;script&gt;alert("XSS")&lt;/script&gt; &amp; &lt;b&gt;bold&lt;/b&gt;'
          );
        });
      });

      it('should handle overlapping highlights correctly', () => {
        const mockResponse = new SearchContentResponse({
          results: [
            new SearchResult({
              identifier: 'overlap-test',
              title: 'Overlap Test',
              fragment: 'overlapping text here',
              highlights: [
                new HighlightSpan({ start: 0, end: 11 }), // "overlapping"
                new HighlightSpan({ start: 12, end: 16 }), // "text"
              ],
            }),
          ],
        });

        searchContentStub.resolves(mockResponse);
        
        return searchClient.search('overlap').then(results => {
          expect(results[0].fragmentHTML).to.equal(
            '<mark>overlapping</mark> <mark>text</mark> here'
          );
        });
      });

      it('should convert newlines to <br> tags', () => {
        const mockResponse = new SearchContentResponse({
          results: [
            new SearchResult({
              identifier: 'multiline',
              title: 'Multiline',
              fragment: 'Line 1\nLine 2\nLine 3',
              highlights: [
                new HighlightSpan({ start: 0, end: 6 }), // "Line 1"
              ],
            }),
          ],
        });

        searchContentStub.resolves(mockResponse);
        
        return searchClient.search('multiline').then(results => {
          expect(results[0].fragmentHTML).to.equal(
            '<mark>Line 1</mark><br>Line 2<br>Line 3'
          );
        });
      });
    });
  });
});