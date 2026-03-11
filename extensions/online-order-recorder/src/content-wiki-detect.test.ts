import { describe, it, expect, beforeEach, vi } from 'vitest';
import { JSDOM } from 'jsdom';

describe('content-wiki-detect', () => {
  let sendMessageMock: ReturnType<typeof vi.fn>;
  let debugMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.resetModules();
    sendMessageMock = vi.fn();
    debugMock = vi.fn();

    (globalThis as Record<string, unknown>)['browser'] = {
      runtime: {
        sendMessage: sendMessageMock,
      },
    };

    (globalThis as Record<string, unknown>)['console'] = {
      ...console,
      debug: debugMock,
    };
  });

  describe('when meta tag with simple-wiki-url exists', () => {
    beforeEach(async () => {
      const dom = new JSDOM(
        '<!DOCTYPE html><html><head><meta name="simple-wiki-url" content="http://wiki.local:8050"></head><body></body></html>'
      );

      (globalThis as Record<string, unknown>)['document'] = dom.window.document;

      // @ts-expect-error TS2306: side-effect-only module has no exports
      await import('./content-wiki-detect.js');
    });

    it('should send WIKI_URL_DETECTED message with the URL', () => {
      expect(sendMessageMock).toHaveBeenCalledWith({
        type: 'WIKI_URL_DETECTED',
        wikiUrl: 'http://wiki.local:8050',
      });
    });

    it('should log the detected URL', () => {
      expect(debugMock).toHaveBeenCalledWith(
        '[Simple Wiki Companion] Wiki URL detected:',
        'http://wiki.local:8050'
      );
    });
  });

  describe('when meta tag exists but content is empty', () => {
    beforeEach(async () => {
      const dom = new JSDOM(
        '<!DOCTYPE html><html><head><meta name="simple-wiki-url" content=""></head><body></body></html>'
      );

      (globalThis as Record<string, unknown>)['document'] = dom.window.document;

      // @ts-expect-error TS2306: side-effect-only module has no exports
      await import('./content-wiki-detect.js');
    });

    it('should not send a message', () => {
      expect(sendMessageMock).not.toHaveBeenCalled();
    });

    it('should log that no wiki meta tag was found', () => {
      expect(debugMock).toHaveBeenCalledWith(
        '[Simple Wiki Companion] No wiki meta tag found on this page'
      );
    });
  });

  describe('when meta tag does not exist', () => {
    beforeEach(async () => {
      const dom = new JSDOM(
        '<!DOCTYPE html><html><head></head><body></body></html>'
      );

      (globalThis as Record<string, unknown>)['document'] = dom.window.document;

      // @ts-expect-error TS2306: side-effect-only module has no exports
      await import('./content-wiki-detect.js');
    });

    it('should not send a message', () => {
      expect(sendMessageMock).not.toHaveBeenCalled();
    });

    it('should log that no wiki meta tag was found', () => {
      expect(debugMock).toHaveBeenCalledWith(
        '[Simple Wiki Companion] No wiki meta tag found on this page'
      );
    });
  });
});
