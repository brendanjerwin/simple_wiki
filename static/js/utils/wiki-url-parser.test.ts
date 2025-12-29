import { expect } from '@open-wc/testing';
import { WikiUrlParser, WikiUrlParseResult } from './wiki-url-parser.js';

type SuccessResult = Extract<WikiUrlParseResult, { success: true }>;
type FailureResult = Extract<WikiUrlParseResult, { success: false }>;

describe('WikiUrlParser', () => {
  describe('parse', () => {
    describe('when given a full URL with command', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('https://wiki.example.com/my_page/view');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given a full URL without command', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('https://wiki.example.com/my_page');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given an absolute path with command', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/my_page/view');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given an absolute path without command', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/my_page');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given just an identifier', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('my_page');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
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
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('/toolbox/edit');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with /raw command', () => {
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('/toolbox/raw');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with /frontmatter command', () => {
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('/toolbox/frontmatter');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });
    });

    describe('when given an empty string', () => {
      let result: FailureResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('');
        if (parsed.success) throw new Error('Expected failure');
        result = parsed;
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });

    describe('when given only whitespace', () => {
      let result: FailureResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('   ');
        if (parsed.success) throw new Error('Expected failure');
        result = parsed;
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });

    describe('when given a URL from a different domain', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('https://other-wiki.com/garage_shelf/view');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
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
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('http://localhost:8050/my_page/view');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
        });

        it('should return success true', () => {
          expect(result.success).to.be.true;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('my_page');
        });
      });

      describe('with domain and non-standard port', () => {
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('https://wiki.example.com:8443/toolbox/edit');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
        });

        it('should return success true', () => {
          expect(result.success).to.be.true;
        });

        it('should extract the page identifier', () => {
          expect(result.pageIdentifier).to.equal('toolbox');
        });
      });

      describe('with IP address and port', () => {
        let result: SuccessResult;

        beforeEach(() => {
          const parsed = WikiUrlParser.parse('http://192.168.1.100:8080/garage_shelf');
          if (!parsed.success) throw new Error('Expected success');
          result = parsed;
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
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/lab_shelf_42/view');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the full identifier', () => {
        expect(result.pageIdentifier).to.equal('lab_shelf_42');
      });
    });

    describe('when given a URL with query parameters', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('https://wiki.example.com/my_page/view?foo=bar');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier ignoring query params', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given a URL with trailing slash', () => {
      let result: SuccessResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/my_page/');
        if (!parsed.success) throw new Error('Expected success');
        result = parsed;
      });

      it('should return success true', () => {
        expect(result.success).to.be.true;
      });

      it('should extract the page identifier', () => {
        expect(result.pageIdentifier).to.equal('my_page');
      });
    });

    describe('when given just a slash', () => {
      let result: FailureResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/');
        if (parsed.success) throw new Error('Expected failure');
        result = parsed;
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include an error message', () => {
        expect(result.error).to.exist;
      });
    });

    describe('when given a malformed URL', () => {
      let result: FailureResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('https://[invalid-url');
        if (parsed.success) throw new Error('Expected failure');
        result = parsed;
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include invalid URL format error', () => {
        expect(result.error).to.equal('Invalid URL format');
      });
    });

    describe('when given a path with invalid page identifier', () => {
      let result: FailureResult;

      beforeEach(() => {
        const parsed = WikiUrlParser.parse('/123_starts_with_number');
        if (parsed.success) throw new Error('Expected failure');
        result = parsed;
      });

      it('should return success false', () => {
        expect(result.success).to.be.false;
      });

      it('should include invalid page identifier error', () => {
        expect(result.error).to.equal('Invalid page identifier: 123_starts_with_number');
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

    describe('when given Unicode identifiers', () => {
      it('should return true for Japanese hiragana', () => {
        expect(WikiUrlParser.isValidPageIdentifier('こんにちは')).to.be.true;
      });

      it('should return true for Chinese characters', () => {
        expect(WikiUrlParser.isValidPageIdentifier('北京市')).to.be.true;
      });

      it('should return true for Arabic text', () => {
        expect(WikiUrlParser.isValidPageIdentifier('مرحبا')).to.be.true;
      });

      it('should return true for Greek letters', () => {
        expect(WikiUrlParser.isValidPageIdentifier('αβγδ')).to.be.true;
      });

      it('should return true for Cyrillic text', () => {
        expect(WikiUrlParser.isValidPageIdentifier('москва')).to.be.true;
      });

      it('should return true for mixed scripts', () => {
        expect(WikiUrlParser.isValidPageIdentifier('hello世界')).to.be.true;
      });

      it('should return true for Unicode with underscores', () => {
        expect(WikiUrlParser.isValidPageIdentifier('東京_駅')).to.be.true;
      });

      it('should return true for Arabic-Indic digits after letter', () => {
        expect(WikiUrlParser.isValidPageIdentifier('عربي٠١٢٣')).to.be.true;
      });

      it('should return true for accented characters', () => {
        expect(WikiUrlParser.isValidPageIdentifier('café')).to.be.true;
      });

      it('should return false for identifier starting with number (any numeral system)', () => {
        // Arabic-Indic digits are numbers, so shouldn't start an identifier
        expect(WikiUrlParser.isValidPageIdentifier('٠١٢٣')).to.be.false;
      });
    });
  });
});
