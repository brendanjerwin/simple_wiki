import { expect, fixture, html } from '@open-wc/testing';
import { stub, type SinonStub } from 'sinon';
import { toTitleCase, type TitleInput } from './title-input.js';
import './title-input.js';

describe('toTitleCase', () => {
  it('should capitalize the first letter of each word', () => {
    expect(toTitleCase('hello world')).to.equal('Hello World');
  });

  it('should lowercase common articles mid-sentence', () => {
    expect(toTitleCase('the lord of the rings')).to.equal('The Lord of the Rings');
  });

  it('should capitalize the last word even if it is an article', () => {
    expect(toTitleCase('something to look at')).to.equal('Something to Look At');
  });

  it('should handle single words', () => {
    expect(toTitleCase('hello')).to.equal('Hello');
  });

  it('should handle empty strings', () => {
    expect(toTitleCase('')).to.equal('');
  });

  it('should preserve spacing', () => {
    expect(toTitleCase('hello   world')).to.equal('Hello   World');
  });

  it('should handle mixed case input', () => {
    expect(toTitleCase('hELLO wORLD')).to.equal('Hello World');
  });

  it('should capitalize first word even if it is an article', () => {
    expect(toTitleCase('a tale of two cities')).to.equal('A Tale of Two Cities');
  });
});

describe('TitleInput', () => {
  let el: TitleInput;
  let fetchStub: SinonStub;

  beforeEach(async () => {
    fetchStub = stub(window, 'fetch');
    fetchStub.resolves(new Response(''));
  });

  afterEach(() => {
    fetchStub.restore();
  });

  it('should exist', async () => {
    el = await fixture(html`<title-input></title-input>`);
    expect(el).to.not.equal(null);
  });

  describe('when text is entered', () => {
    let input: HTMLInputElement;

    beforeEach(async () => {
      el = await fixture(html`<title-input></title-input>`);
      input = el.shadowRoot!.querySelector('input')!;
      input.value = 'hello world';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await el.updateComplete;
    });

    it('should title-case the value', () => {
      expect(el.value).to.equal('Hello World');
    });

    it('should update the input element', () => {
      expect(input.value).to.equal('Hello World');
    });
  });

  describe('when input is blurred with leading/trailing spaces', () => {
    let input: HTMLInputElement;

    beforeEach(async () => {
      el = await fixture(html`<title-input></title-input>`);
      input = el.shadowRoot!.querySelector('input')!;
      input.value = '  hello world  ';
      input.dispatchEvent(new Event('input', { bubbles: true }));
      await el.updateComplete;
      input.dispatchEvent(new Event('blur', { bubbles: true }));
      await el.updateComplete;
    });

    it('should trim the value', () => {
      expect(el.value).to.equal('Hello World');
    });
  });

  describe('when disabled', () => {
    let input: HTMLInputElement;

    beforeEach(async () => {
      el = await fixture(html`<title-input disabled></title-input>`);
      input = el.shadowRoot!.querySelector('input')!;
    });

    it('should disable the input element', () => {
      expect(input.disabled).to.equal(true);
    });
  });

  describe('when placeholder is set', () => {
    let input: HTMLInputElement;

    beforeEach(async () => {
      el = await fixture(html`<title-input placeholder="Enter title here"></title-input>`);
      input = el.shadowRoot!.querySelector('input')!;
    });

    it('should set the placeholder on the input', () => {
      expect(input.placeholder).to.equal('Enter title here');
    });
  });
});
