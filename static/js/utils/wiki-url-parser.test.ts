import { expect } from '@open-wc/testing';
import { WikiUrlParser, WikiUrlParseResult } from './wiki-url-parser.js';

describe('WikiUrlParser', () => {
  describe('parse', () => {
    describe('when given a full URL with command', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('https://wiki.example.com/my_page/view');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given a full URL without command', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('https://wiki.example.com/my_page');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given an absolute path with command', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('/my_page/view');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given an absolute path without command', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('/my_page');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given just an identifier', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('my_page');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should return the identifier as-is', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given a URL with different commands', () => {
      describe('with /edit command', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('/toolbox/edit');
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with /raw command', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('/toolbox/raw');
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with /frontmatter command', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('/toolbox/frontmatter');
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });
    });

    describe('when given an empty string', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });

    describe('when given only whitespace', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('   ');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });

    describe('when given a URL from a different domain', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('https://other-wiki.com/garage_shelf/view');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should still parse the path portion', () => {
        expect(result.pageIdentifier).to.equal('garage_shelf');
      });
    });

    describe('when given a URL with a port number', () => {
      describe('with localhost and port', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('http://localhost:8050/my_page/view');
        });

        it('should return success true', () => {
          expect(result.success).to.be.true;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('my_page');
        });
      });

      describe('with domain and non-standard port', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('https://wiki.example.com:8443/toolbox/edit');
        });

        it('should return success true', () => {
          expect(result.success).to.be.true;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with IP address and port', () => {
        let result: WikiUrlParseResult;

        beforeEach(() => {
          result = WikiUrlParser.parse('http://192.168.1.100:8080/garage_shelf');
        });

        it('should return success true', () => {
          expect(result.success).to.be.true;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('garage_shelf');
        });
      });
    });

    describe('when given a complex identifier with underscores and numbers', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('/lab_shelf_42/view');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the full identifier', () => {
        expect(result.pageIdentifier).to.equal('lab_shelf_42');
      });
    });

    describe('when given a URL with query parameters', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('https://wiki.example.com/my_page/view?foo=bar');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier ignoring query params', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given a URL with trailing slash', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('/my_page/');
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given just a slash', () => {
      let result: WikiUrlParseResult;

      beforeEach(() => {
        result = WikiUrlParser.parse('/');
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });
  });

  describe('isValidPageIdentifier', () => {
    describe('when given a valid snake_case identifier', () => {
      it('should return true for simple identifier', () => {
        expect(WikiUrlParser.isValidPageIdentifier('my_page')).to.be.true;
      });

      it('should return true for identifier with numbers', () => {
        expect(WikiUrlParser.isValidPageIdentifier('tool_box_123')).to.be.true;
      });

      it('should return true for single word', () => {
        expect(WikiUrlParser.isValidPageIdentifier('toolbox')).to.be.true;
      });
    });

    describe('when given an invalid identifier', () => {
      it('should return false for empty string', () => {
        expect(WikiUrlParser.isValidPageIdentifier('')).to.be.false;
      });

      it('should return false for identifier with spaces', () => {
        expect(WikiUrlParser.isValidPageIdentifier('my page')).to.be.false;
      });

      it('should return false for identifier with special characters', () => {
        expect(WikiUrlParser.isValidPageIdentifier('my@page')).to.be.false;
      });

      it('should return false for identifier starting with number', () => {
        expect(WikiUrlParser.isValidPageIdentifier('123_page')).to.be.false;
      });
    });

    describe('when given identifiers with allowed characters', () => {
      it('should return true for identifier with hyphens', () => {
        // Hyphens are common in URLs, should be allowed
        expect(WikiUrlParser.isValidPageIdentifier('my-page')).to.be.true;
      });

      it('should return true for uppercase letters', () => {
        // URLs can have uppercase, will be normalized by backend
        expect(WikiUrlParser.isValidPageIdentifier('MyPage')).to.be.true;
      });
    });
  });
});
