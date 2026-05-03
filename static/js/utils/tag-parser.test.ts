import { expect } from '@open-wc/testing';
import { extractHashtags, normalizeHashtag, MAX_TAG_LEN } from './tag-parser.js';

describe('normalizeHashtag', () => {
  describe('when input is plain ASCII', () => {
    it('should lowercase the tag', () => {
      expect(normalizeHashtag('Groceries')).to.equal('groceries');
    });
  });

  describe('when input contains hyphen and underscore', () => {
    it('should preserve hyphens', () => {
      expect(normalizeHashtag('home-lab')).to.equal('home-lab');
    });

    it('should preserve underscores', () => {
      expect(normalizeHashtag('home_lab')).to.equal('home_lab');
    });

    it('should keep hyphens and underscores distinct', () => {
      expect(normalizeHashtag('home-lab')).to.not.equal(normalizeHashtag('home_lab'));
    });
  });

  describe('when input contains stylized Unicode', () => {
    it('should NFKC-fold compatibility forms', () => {
      // ＡＢＣ (fullwidth) -> ABC -> abc
      expect(normalizeHashtag('ＡＢＣ')).to.equal('abc');
    });
  });

  describe('when input contains disallowed characters', () => {
    it('should drop punctuation', () => {
      expect(normalizeHashtag('foo!bar')).to.equal('foobar');
    });
  });

  describe('when input is longer than the cap', () => {
    let result: string;

    beforeEach(() => {
      const input = 'a'.repeat(MAX_TAG_LEN + 10);
      result = normalizeHashtag(input);
    });

    it('should truncate to MAX_TAG_LEN runes', () => {
      expect(Array.from(result)).to.have.length(MAX_TAG_LEN);
    });
  });
});

describe('extractHashtags', () => {
  describe('when body has no hashtags', () => {
    it('should return an empty array', () => {
      expect(extractHashtags('just plain text')).to.deep.equal([]);
    });
  });

  describe('when body has a single hashtag', () => {
    it('should return the normalized tag', () => {
      expect(extractHashtags('buy #milk today')).to.deep.equal(['milk']);
    });
  });

  describe('when hashtag begins the body', () => {
    it('should match at start of string', () => {
      expect(extractHashtags('#urgent: ship it')).to.deep.equal(['urgent']);
    });
  });

  describe('when hashtag follows whitespace', () => {
    it('should match after a space', () => {
      expect(extractHashtags('a #bee c')).to.deep.equal(['bee']);
    });
  });

  describe('when body has multiple distinct hashtags', () => {
    it('should return them in first-occurrence order', () => {
      expect(extractHashtags('#alpha and #beta and #gamma')).to.deep.equal(['alpha', 'beta', 'gamma']);
    });
  });

  describe('when body has duplicate hashtags', () => {
    it('should deduplicate to first occurrence', () => {
      expect(extractHashtags('#dup once #dup twice')).to.deep.equal(['dup']);
    });
  });

  describe('when hashtag is mid-word', () => {
    it('should not extract', () => {
      expect(extractHashtags('foo#bar')).to.deep.equal([]);
    });
  });

  describe('when content is a markdown anchor link', () => {
    it('should not extract the anchor as a tag', () => {
      expect(extractHashtags('see [link](#section) for more')).to.deep.equal([]);
    });
  });

  describe('when hashtag is escaped', () => {
    it('should not extract \\#tag', () => {
      expect(extractHashtags('literal \\#hash here')).to.deep.equal([]);
    });
  });

  describe('when hashtag is inside an inline code span', () => {
    it('should not extract', () => {
      expect(extractHashtags('code: `not a #tag` here')).to.deep.equal([]);
    });
  });

  describe('when hashtag is inside a fenced code block', () => {
    it('should not extract from inside the fence', () => {
      const body = 'before\n```\n#fence-tag\n```\nafter #real';
      expect(extractHashtags(body)).to.deep.equal(['real']);
    });
  });

  describe('when hashtag normalization differs from raw spelling', () => {
    it('should return the normalized form', () => {
      expect(extractHashtags('#Groceries')).to.deep.equal(['groceries']);
    });
  });

  describe('when tag is numeric only', () => {
    it('should accept numeric tags', () => {
      expect(extractHashtags('year #2026 wraps up')).to.deep.equal(['2026']);
    });
  });

  describe('when tag contains hyphen and underscore', () => {
    it('should preserve them', () => {
      expect(extractHashtags('setup #home-lab and #project_alpha')).to.deep.equal(['home-lab', 'project_alpha']);
    });
  });

  describe('when input is `#` followed by space', () => {
    it('should not extract an empty tag', () => {
      expect(extractHashtags('# header-ish')).to.deep.equal([]);
    });
  });

  describe('when extraction is run twice on the same body', () => {
    it('should be idempotent for repeated calls', () => {
      const body = 'a #foo b #bar';
      const first = extractHashtags(body);
      const second = extractHashtags(body);
      expect(second).to.deep.equal(first);
    });
  });
});
