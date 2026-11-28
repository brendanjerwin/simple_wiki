import { expect } from '@open-wc/testing';
import { stub, restore, type SinonStub } from 'sinon';
import {
  attributeErrorEvent,
  attributeRejection,
  logForeignError,
  leadingUrlToken,
} from './error-attribution.js';
import type { ErrorAttribution } from './error-attribution.js';

// The canonical #1501 stack: a rejection dispatched from a Safari extension's
// masked content script. Our own frames sit on top; the foreign frames are
// below `dispatchEvent`.
const FOREIGN_ENTRY_STACK = [
  'Error',
  '    at rejectionHandler (/static/js/web-components/global-error-handler.ts:1:1)',
  '    at dispatchEvent@[native code]',
  '    at e@webkit-masked-url://hidden/:54:12285',
  '    at I@webkit-masked-url://hidden/:54:13777',
  '    at te@webkit-masked-url://hidden/:54:18731',
  '    at @webkit-masked-url://hidden/:54:10017',
  '    at r@webkit-masked-url://hidden/:54:9177',
  '    at r@webkit-masked-url://hidden/:2:425300',
].join('\n');

const OWN_ENTRY_STACK = [
  'Error',
  '    at rejectionHandler (/static/js/web-components/global-error-handler.ts:1:1)',
  '    at /static/js/dist/web-components.js:67927:60',
].join('\n');

const OWN_REASON_STACK = [
  'Error: page action failed',
  '    at Object.action (/static/js/dist/web-components.js:16624:35)',
  '    at /static/js/dist/web-components.js:67927:60',
].join('\n');

const FOREIGN_REASON_STACK = [
  'Error: extension blew up',
  '    at e@webkit-masked-url://hidden/:54:12285',
  '    at I@webkit-masked-url://hidden/:54:13777',
].join('\n');

const UNATTRIBUTABLE_STACK = ['Error: mystery', '    at <anonymous>'].join('\n');

describe('attributeErrorEvent', () => {
  describe('when the event names a webkit-masked-url script', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: 'webkit-masked-url://hidden/:54:12285',
        error: new Error('thrown by an extension'),
      });
    });

    it('should attribute the error to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });

    it('should cite the masked script in the rationale', () => {
      expect(attribution.rationale).to.contain('webkit-masked-url://hidden/');
    });
  });

  describe('when the event names a Safari extension script', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: 'safari-web-extension://ABC-123/content.js:12:4',
        error: null,
      });
    });

    it('should attribute the error to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when the event names a script on another origin', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: 'https://cdn.example.com/widget.js:1:1',
        error: null,
      });
    });

    it('should attribute the error to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when the event names one of our own scripts', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: `${globalThis.location.origin}/static/js/dist/web-components.js:16624:35`,
        error: new Error('our own bug'),
      });
    });

    it('should attribute the error to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the event names an origin-less same-origin script path', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: '/static/js/dist/web-components.js:16624:35',
        error: null,
      });
    });

    it('should attribute the error to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the event has no filename but a foreign stack', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: '',
        error: { stack: FOREIGN_REASON_STACK },
      });
    });

    it('should attribute the error to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when the event has no filename and an own stack', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: '',
        error: { stack: OWN_REASON_STACK },
      });
    });

    it('should attribute the error to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the event carries no usable evidence', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeErrorEvent({
        filename: 'Script error.',
        error: null,
      });
    });

    it('should default to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });

    it('should say the error was unattributable', () => {
      expect(attribution.rationale).to.contain('no usable attribution evidence');
    });
  });
});

describe('attributeRejection', () => {
  describe('when the reason is an Error thrown by our own code', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: { name: 'Error', message: 'boom', stack: OWN_REASON_STACK },
        handlerEntryStack: OWN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the reason is an Error thrown by a masked script', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: { name: 'Error', message: 'boom', stack: FOREIGN_REASON_STACK },
        handlerEntryStack: OWN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when a foreign script dispatched the rejection with an undefined reason', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: undefined,
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });

    it('should cite the masked dispatch frame in the rationale', () => {
      expect(attribution.rationale).to.contain('webkit-masked-url://hidden/');
    });
  });

  describe('when a foreign script dispatched the rejection with an empty-object reason', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: {},
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when a foreign script dispatched the rejection with a null reason', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: null,
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when the reason has an unattributable stack but the event was dispatched by a foreign script', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: { name: 'Error', message: 'mystery', stack: UNATTRIBUTABLE_STACK },
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });

  describe('when the reason has no stack and the handler was called from our own code', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: 'String rejection reason',
        handlerEntryStack: OWN_ENTRY_STACK,
      });
    });

    it('should default to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when neither the reason nor the handler entry stack is usable', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: undefined,
        handlerEntryStack: undefined,
      });
    });

    it('should default to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the reason belongs to our own code but a foreign script dispatched it', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: { name: 'Error', message: 'boom', stack: OWN_REASON_STACK },
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
    });

    it('should trust the reason stack and attribute the rejection to our own code', () => {
      expect(attribution.origin).to.equal('ours');
    });
  });

  describe('when the reason is a cross-origin Error', () => {
    let attribution: ErrorAttribution;

    beforeEach(() => {
      attribution = attributeRejection({
        reason: {
          name: 'Error',
          message: 'boom',
          stack: 'Error: boom\n    at https://cdn.example.com/widget.js:1:1',
        },
        handlerEntryStack: OWN_ENTRY_STACK,
      });
    });

    it('should attribute the rejection to foreign code', () => {
      expect(attribution.origin).to.equal('foreign');
    });
  });
});

describe('leadingUrlToken', () => {
  // Table-driven for the three documented shapes (and their variations).
  interface TokenCase {
    label: string;
    input: string;
    expected: string | undefined;
  }

  const tokenCases: TokenCase[] = [
    {
      label: 'https URL with path and line/col',
      input: 'https://host/path:1:2',
      expected: 'https://host/path:1:2',
    },
    {
      label: 'webkit-masked-url with line/col',
      input: 'webkit-masked-url://hidden/:54:1',
      expected: 'webkit-masked-url://hidden/:54:1',
    },
    {
      label: 'protocol-relative URL (no scheme)',
      input: '//host/x.js',
      expected: '//host/x.js',
    },
    {
      label: 'URL embedded in a stack frame after @',
      input: 'e@webkit-masked-url://hidden/:54:12285',
      expected: 'webkit-masked-url://hidden/:54:12285',
    },
    {
      label: 'cross-origin https URL in a stack frame',
      input: '    at Object.action (https://cdn.example.com/widget.js:1:1)',
      expected: 'https://cdn.example.com/widget.js:1:1',
    },
    {
      label: 'relative path with no //',
      input: '/static/js/dist/web-components.js:16624:35',
      expected: undefined,
    },
    {
      label: 'empty string',
      input: '',
      expected: undefined,
    },
  ];

  for (const { label, input, expected } of tokenCases) {
    describe(`when given a ${label}`, () => {
      let result: string | undefined;

      beforeEach(() => {
        result = leadingUrlToken(input);
      });

      it(`should return ${expected === undefined ? 'undefined' : `"${expected}"`}`, () => {
        expect(result).to.equal(expected);
      });
    });
  }

  describe('when given a long adversarial string with scheme-like prefixes but no //', () => {
    let result: string | undefined;

    beforeEach(() => {
      // 50 k chars: alternating "a:" and "a" — resembles a scheme prefix but
      // has no "//", which caused O(n²) backtracking with the old regex (S8786).
      const adversarial = ('a:' + 'a').repeat(16667); // ~50 k chars

      result = leadingUrlToken(adversarial);
    });

    it('should return undefined', () => {
      expect(result).to.be.undefined;
    });
  });
});

describe('logForeignError', () => {
  let warnStub: SinonStub;

  beforeEach(() => {
    warnStub = stub(console, 'warn');
  });

  afterEach(() => {
    restore();
  });

  describe('when logging an undefined rejection reason', () => {
    let details: Record<string, unknown>;

    beforeEach(() => {
      logForeignError({
        handler: 'unhandledrejection',
        attribution: {
          origin: 'foreign',
          rationale: 'rejection was dispatched from a foreign script: e@webkit-masked-url://hidden/:54:12285',
        },
        reason: undefined,
        isTrusted: false,
        hasPromise: false,
        handlerEntryStack: FOREIGN_ENTRY_STACK,
      });
      details = warnStub.firstCall.args[1] as Record<string, unknown>;
    });

    it('should name the handler in the warning message', () => {
      expect(warnStub.firstCall.args[0]).to.contain('unhandledrejection');
    });

    it('should record the reason type', () => {
      expect(details['reasonType']).to.equal('undefined');
    });

    it('should record that the event was not trusted', () => {
      expect(details['isTrusted']).to.equal(false);
    });

    it('should record the handler entry stack', () => {
      expect(details['handlerEntryStack']).to.equal(FOREIGN_ENTRY_STACK);
    });
  });

  describe('when the reason cannot be stringified', () => {
    let details: Record<string, unknown>;

    beforeEach(() => {
      logForeignError({
        handler: 'error',
        attribution: {
          origin: 'foreign',
          rationale: 'error event filename points at a foreign script: webkit-masked-url://hidden/:54:1',
        },
        reason: {
          toString: (): string => {
            throw new TypeError('cannot stringify');
          },
        },
        isTrusted: true,
        hasPromise: false,
        handlerEntryStack: undefined,
      });
      details = warnStub.firstCall.args[1] as Record<string, unknown>;
    });

    it('should not throw', () => {
      expect(warnStub).to.have.been.calledOnce;
    });

    it('should mark the reason as unstringifiable', () => {
      expect(details['reasonString']).to.equal('<unstringifiable>');
    });
  });
});
