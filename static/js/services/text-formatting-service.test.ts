import { expect } from '@open-wc/testing';
import { TextFormattingService, FormattingResult } from './text-formatting-service.js';

describe('TextFormattingService', () => {
  let service: TextFormattingService;

  beforeEach(() => {
    service = new TextFormattingService();
  });

  it('should exist', () => {
    expect(service).to.exist;
  });

  describe('wrapBold', () => {
    describe('when text is selected', () => {
      let result: FormattingResult;
      const text = 'Hello world!';
      const selectionStart = 6;
      const selectionEnd = 11;

      beforeEach(() => {
        // "world" is selected
        result = service.wrapBold(text, selectionStart, selectionEnd);
      });

      it('should wrap selection in **', () => {
        expect(result.newText).to.equal('Hello **world**!');
      });

      it('should position selection after opening markers', () => {
        expect(result.newSelectionStart).to.equal(8);
      });

      it('should position selection end before closing markers', () => {
        expect(result.newSelectionEnd).to.equal(13);
      });
    });

    describe('when no text is selected (cursor only)', () => {
      let result: FormattingResult;
      const text = 'Hello world!';
      const cursorPos = 6;

      beforeEach(() => {
        result = service.wrapBold(text, cursorPos, cursorPos);
      });

      it('should insert **bold** placeholder', () => {
        expect(result.newText).to.equal('Hello **bold**world!');
      });

      it('should select the placeholder text', () => {
        expect(result.newSelectionStart).to.equal(8);
        expect(result.newSelectionEnd).to.equal(12);
      });
    });

    describe('when selection is at beginning of text', () => {
      let result: FormattingResult;

      beforeEach(() => {
        result = service.wrapBold('Hello', 0, 5);
      });

      it('should wrap correctly', () => {
        expect(result.newText).to.equal('**Hello**');
      });
    });

    describe('when selection is at end of text', () => {
      let result: FormattingResult;

      beforeEach(() => {
        result = service.wrapBold('Hello world', 6, 11);
      });

      it('should wrap correctly', () => {
        expect(result.newText).to.equal('Hello **world**');
      });
    });
  });

  describe('wrapItalic', () => {
    describe('when text is selected', () => {
      let result: FormattingResult;
      const text = 'Hello world!';
      const selectionStart = 6;
      const selectionEnd = 11;

      beforeEach(() => {
        result = service.wrapItalic(text, selectionStart, selectionEnd);
      });

      it('should wrap selection in *', () => {
        expect(result.newText).to.equal('Hello *world*!');
      });

      it('should position selection after opening marker', () => {
        expect(result.newSelectionStart).to.equal(7);
      });

      it('should position selection end before closing marker', () => {
        expect(result.newSelectionEnd).to.equal(12);
      });
    });

    describe('when no text is selected (cursor only)', () => {
      let result: FormattingResult;
      const text = 'Hello world!';
      const cursorPos = 6;

      beforeEach(() => {
        result = service.wrapItalic(text, cursorPos, cursorPos);
      });

      it('should insert *italic* placeholder', () => {
        expect(result.newText).to.equal('Hello *italic*world!');
      });

      it('should select the placeholder text', () => {
        expect(result.newSelectionStart).to.equal(7);
        expect(result.newSelectionEnd).to.equal(13);
      });
    });
  });

  describe('insertLink', () => {
    describe('when text is selected', () => {
      let result: FormattingResult;
      const text = 'Click here for info';
      const selectionStart = 6;
      const selectionEnd = 10;

      beforeEach(() => {
        // "here" is selected
        result = service.insertLink(text, selectionStart, selectionEnd);
      });

      it('should wrap selection in markdown link syntax', () => {
        expect(result.newText).to.equal('Click [here](url) for info');
      });

      it('should select the url placeholder', () => {
        expect(result.newSelectionStart).to.equal(13);
        expect(result.newSelectionEnd).to.equal(16);
      });
    });

    describe('when text is selected with url provided', () => {
      let result: FormattingResult;
      const text = 'Click here for info';
      const selectionStart = 6;
      const selectionEnd = 10;

      beforeEach(() => {
        result = service.insertLink(text, selectionStart, selectionEnd, 'https://example.com');
      });

      it('should use the provided url', () => {
        expect(result.newText).to.equal('Click [here](https://example.com) for info');
      });

      it('should position cursor after the link', () => {
        expect(result.newSelectionStart).to.equal(33);
        expect(result.newSelectionEnd).to.equal(33);
      });
    });

    describe('when no text is selected (cursor only)', () => {
      let result: FormattingResult;
      const text = 'Some text';
      const cursorPos = 5;

      beforeEach(() => {
        result = service.insertLink(text, cursorPos, cursorPos);
      });

      it('should insert [link text](url) placeholder', () => {
        expect(result.newText).to.equal('Some [link text](url)text');
      });

      it('should select the link text placeholder', () => {
        expect(result.newSelectionStart).to.equal(6);
        expect(result.newSelectionEnd).to.equal(15);
      });
    });

    describe('when no text is selected with url provided', () => {
      let result: FormattingResult;
      const text = 'Some text';
      const cursorPos = 5;

      beforeEach(() => {
        result = service.insertLink(text, cursorPos, cursorPos, 'https://example.com');
      });

      it('should insert link with provided url', () => {
        expect(result.newText).to.equal('Some [link text](https://example.com)text');
      });

      it('should select the link text placeholder', () => {
        expect(result.newSelectionStart).to.equal(6);
        expect(result.newSelectionEnd).to.equal(15);
      });
    });
  });
});
