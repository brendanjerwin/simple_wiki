import { expect } from '@open-wc/testing';
import { parseTaggedInput, composeTaggedText } from './checklist-tag-parser.js';
import type { ChecklistItem } from './checklist-tag-parser.js';

describe('parseTaggedInput', () => {
  describe('when input has a single #tag at the start', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('#Dairy Buy milk');
    });

    it('should extract the tag lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy']);
    });

    it('should extract the text after the tag', () => {
      expect(result.text).to.equal('Buy milk');
    });
  });

  describe('when input has a single #tag at the end', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('Buy milk #Dairy');
    });

    it('should extract the tag lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy']);
    });

    it('should extract the text without the tag', () => {
      expect(result.text).to.equal('Buy milk');
    });
  });

  describe('when input has a #tag in the middle', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('buy #dairy milk');
    });

    it('should extract the tag lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy']);
    });

    it('should join the remaining text', () => {
      expect(result.text).to.equal('buy milk');
    });
  });

  describe('when input has multiple tags', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('milk #dairy #fridge');
    });

    it('should extract all tags lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy', 'fridge']);
    });

    it('should extract the remaining text', () => {
      expect(result.text).to.equal('milk');
    });
  });

  describe('when input has multiple tags scattered throughout', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('#dairy milk #fridge');
    });

    it('should extract all tags lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy', 'fridge']);
    });

    it('should extract the remaining text', () => {
      expect(result.text).to.equal('milk');
    });
  });

  describe('when input has no tag', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('Buy milk');
    });

    it('should have empty tags array', () => {
      expect(result.tags).to.deep.equal([]);
    });

    it('should use the full input as text', () => {
      expect(result.text).to.equal('Buy milk');
    });
  });

  describe('when input has #tag but no item text', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('#Dairy');
    });

    it('should extract the tag lowercased', () => {
      expect(result.tags).to.deep.equal(['dairy']);
    });

    it('should have empty text', () => {
      expect(result.text).to.equal('');
    });
  });

  describe('when input has mixed case tag', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('milk #DAIRY');
    });

    it('should lowercase the tag', () => {
      expect(result.tags).to.deep.equal(['dairy']);
    });

    it('should extract the remaining text', () => {
      expect(result.text).to.equal('milk');
    });
  });

  describe('when input is empty', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('');
    });

    it('should return empty tags', () => {
      expect(result.tags).to.deep.equal([]);
    });

    it('should return empty text', () => {
      expect(result.text).to.equal('');
    });
  });

  describe('when input has only whitespace', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('   ');
    });

    it('should return empty tags', () => {
      expect(result.tags).to.deep.equal([]);
    });

    it('should return empty text', () => {
      expect(result.text).to.equal('');
    });
  });

  describe('when input has three tags', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('eggs #dairy #fridge #breakfast');
    });

    it('should extract all three tags', () => {
      expect(result.tags).to.deep.equal(['dairy', 'fridge', 'breakfast']);
    });

    it('should extract the remaining text', () => {
      expect(result.text).to.equal('eggs');
    });
  });

  describe('when input has a hyphenated tag', () => {
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('reset router #home-lab');
    });

    it('should preserve the hyphen in the tag', () => {
      expect(result.tags).to.deep.equal(['home-lab']);
    });

    it('should extract the text without the tag', () => {
      expect(result.text).to.equal('reset router');
    });
  });

  describe('when input has a # with a digit followed by space (not a tag)', () => {
    // Mid-word `item#5` shouldn't extract — no preceding space.
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('Buy item#5 of these');
    });

    it('should not extract anything as a tag', () => {
      expect(result.tags).to.deep.equal([]);
    });

    it('should keep the text intact', () => {
      expect(result.text).to.equal('Buy item#5 of these');
    });
  });

  describe('when input has a backslash-escaped #', () => {
    // `\#5` is an escape — the `#5` should appear in the text but NOT
    // be extracted as a tag. The backslash itself is consumed.
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('Buy \\#5 of these #urgent');
    });

    it('should extract only the unescaped #urgent tag', () => {
      expect(result.tags).to.deep.equal(['urgent']);
    });

    it('should preserve the escaped # in the text without the backslash', () => {
      expect(result.text).to.equal('Buy #5 of these');
    });
  });

  describe('when a # is inside an inline code span', () => {
    // The `#notreal` inside backticks should NOT be extracted and should
    // remain visible in the text exactly as written.
    let result: ReturnType<typeof parseTaggedInput>;

    beforeEach(() => {
      result = parseTaggedInput('see `example #notreal` and #real');
    });

    it('should extract only the #real tag from outside the code span', () => {
      expect(result.tags).to.deep.equal(['real']);
    });

    it('should preserve the in-code-span # in the text', () => {
      expect(result.text).to.equal('see `example #notreal` and');
    });
  });
});

describe('composeTaggedText', () => {
  describe('when item has tags', () => {
    let result: string;

    beforeEach(() => {
      const item: ChecklistItem = { text: 'milk', checked: false, tags: ['dairy', 'fridge'] };
      result = composeTaggedText(item);
    });

    it('should append tags with #tag syntax', () => {
      expect(result).to.equal('milk #dairy #fridge');
    });
  });

  describe('when item has no tags', () => {
    let result: string;

    beforeEach(() => {
      const item: ChecklistItem = { text: 'milk', checked: false, tags: [] };
      result = composeTaggedText(item);
    });

    it('should return just the text', () => {
      expect(result).to.equal('milk');
    });
  });

  describe('when item has a single tag', () => {
    let result: string;

    beforeEach(() => {
      const item: ChecklistItem = { text: 'eggs', checked: true, tags: ['dairy'] };
      result = composeTaggedText(item);
    });

    it('should append the single tag', () => {
      expect(result).to.equal('eggs #dairy');
    });
  });

  describe('when item text is empty', () => {
    let result: string;

    beforeEach(() => {
      const item: ChecklistItem = { text: '', checked: false, tags: ['dairy'] };
      result = composeTaggedText(item);
    });

    it('should return just the tag with leading space', () => {
      expect(result).to.equal(' #dairy');
    });
  });

  describe('when item is checked with multiple tags', () => {
    let result: string;

    beforeEach(() => {
      const item: ChecklistItem = { text: 'yogurt', checked: true, tags: ['dairy', 'fridge', 'breakfast'] };
      result = composeTaggedText(item);
    });

    it('should include all tags in order', () => {
      expect(result).to.equal('yogurt #dairy #fridge #breakfast');
    });
  });
});
