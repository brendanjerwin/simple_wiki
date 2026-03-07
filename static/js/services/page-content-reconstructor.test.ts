import { expect } from '@open-wc/testing';
import { reconstructWholePageText } from './page-content-reconstructor.js';

describe('reconstructWholePageText', () => {
  describe('when both frontmatter and content are empty', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText('', '');
    });

    it('should return an empty string', () => {
      expect(result).to.equal('');
    });
  });

  describe('when frontmatter is empty and content has text', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText('', '# Hello World\n\nSome content.');
    });

    it('should return just the content', () => {
      expect(result).to.equal('# Hello World\n\nSome content.');
    });
  });

  describe('when frontmatter has content and markdown is empty', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText('title = "My Page"\n', '');
    });

    it('should wrap frontmatter in +++ delimiters with trailing newline', () => {
      expect(result).to.equal('+++\ntitle = "My Page"\n+++\n');
    });
  });

  describe('when both frontmatter and content are present', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText(
        'title = "My Page"\ntags = ["wiki", "test"]\n',
        '# Hello World\n\nSome content.'
      );
    });

    it('should combine frontmatter and content with proper delimiters', () => {
      expect(result).to.equal(
        '+++\ntitle = "My Page"\ntags = ["wiki", "test"]\n+++\n# Hello World\n\nSome content.'
      );
    });
  });

  describe('when frontmatter has no trailing newline', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText('title = "Test"', '# Content');
    });

    it('should still produce valid delimited frontmatter', () => {
      expect(result).to.equal('+++\ntitle = "Test"\n+++\n# Content');
    });
  });

  describe('when content has leading newlines', () => {
    let result: string;

    beforeEach(() => {
      result = reconstructWholePageText('title = "Test"\n', '\n\n# Content');
    });

    it('should preserve the leading newlines in content', () => {
      expect(result).to.equal('+++\ntitle = "Test"\n+++\n\n\n# Content');
    });
  });
});
