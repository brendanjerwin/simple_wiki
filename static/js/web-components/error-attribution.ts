/**
 * Attribution for the global error handlers (issue #1501).
 *
 * The wiki replaces the whole page with the Kernel Panic screen on unhandled
 * errors - but only for errors that originate in the wiki's own code. Browser
 * extensions, injected helpers and other masked-URL scripts run in the same
 * page and can reject promises; those failures belong to their authors.
 * Panicking on them turns a third party's bug into a full-page outage for us
 * (issue #1501, 2026-11-28T02:26Z: a rejection dispatched from
 * `webkit-masked-url://hidden/` frames with an `undefined` reason).
 *
 * Verdicts come from stack evidence only:
 * - a frame in a foreign scheme (Safari masks extension scripts as
 *   `webkit-masked-url://hidden/...`) or on another origin is foreign;
 * - a frame in our own origin is ours;
 * - anything else is unattributable and therefore ours. Doubt never resolves
 *   toward silence: our own failures must stay loud (owner ruling on #1501).
 */

/** Who an unhandled error belongs to. */
export type ErrorOrigin = 'ours' | 'foreign';

/** A verdict plus the evidence (or lack of it) behind it. */
export interface ErrorAttribution {
 readonly origin: ErrorOrigin;
 /**
  * Human-readable justification. Included in the foreign-error log so the
  * next incident report names why the page stayed alive.
  */
 readonly rationale: string;
}

type FrameOrigin = ErrorOrigin | 'unknown';

/**
 * Script schemes that are never the wiki's own page code: browser extensions
 * and browser-injected helpers. Safari hides the real URL of these scripts and
 * reports `webkit-masked-url://hidden/...` instead.
 */
const FOREIGN_SCRIPT_SCHEMES: Readonly<Record<string, true>> = {
 'webkit-masked-url': true,
 'safari-web-extension': true,
 'safari-extension': true,
 'safari-extension-content': true,
 'chrome-extension': true,
 'moz-extension': true,
 'ms-browser-extension': true,
 chrome: true,
 resource: true,
 jar: true,
};

/**
 * Extract the leading URL-shaped token from a stack-frame reference.
 *
 * Handles three shapes:
 *  - `https://host/path:1:2`          — scheme + authority
 *  - `webkit-masked-url://hidden/:54:1` — scheme + authority
 *  - `//host/x.js`                    — protocol-relative (no scheme)
 *
 * Uses a linear scan instead of a regex to avoid O(n²) backtracking when a
 * long line contains a scheme-like prefix but no `//` (S8786).
 *
 * Exported for unit testing only.
 */
export function leadingUrlToken(reference: string): string | undefined {
 const sep = reference.indexOf('//');
 if (sep === -1) return undefined;
 // Walk back over a scheme, if any: [A-Za-z][A-Za-z0-9+.-]*:
 let start = sep;
 if (sep > 0 && reference.charAt(sep - 1) === ':') {
  let i = sep - 2;
  while (i >= 0 && /[A-Za-z0-9+.-]/.test(reference.charAt(i))) i--;
  const schemeStart = i + 1;
  // Scheme must have at least one char and start with a letter (RFC 3986).
  if (schemeStart < sep - 1 && /[A-Za-z]/.test(reference.charAt(schemeStart))) {
   start = schemeStart;
  }
 }
 let end = sep + 2;
 while (end < reference.length && !/[\s@()]/.test(reference.charAt(end))) end++;
 // Require at least one character after '//', matching the original regex.
 if (end === sep + 2) return undefined;
 return reference.slice(start, end);
}

/** The scheme of an absolute URL, or `undefined` for protocol-relative URLs. */
const URL_SCHEME_PATTERN = /^([a-zA-Z][a-zA-Z0-9+.-]*):/;

/**
 * A same-origin script frame written without its origin, e.g.
 * `/static/js/dist/web-components.js:16624:35` or `web-components.js:1:2`.
 */
const RELATIVE_SCRIPT_FRAME_PATTERN =
 /(?:^|[\s(@])((?:\.{0,2}\/)?[A-Za-z0-9_$~%./-]*\.(?:m?js|jsx|ts|tsx):\d+:\d+)/u;

function compareUrlToPageOrigin(url: string, pageOrigin: string): FrameOrigin {
 try {
  return new URL(url, globalThis.location.href).origin === pageOrigin
   ? 'ours'
   : 'foreign';
 } catch {
  return 'unknown';
 }
}

/**
 * Classify one reference - a stack frame or an `ErrorEvent.filename` - by the
 * script it points at.
 */
function classifyReference(reference: string, pageOrigin: string): FrameOrigin {
 const absoluteUrl = leadingUrlToken(reference);
 if (absoluteUrl !== undefined) {
  const scheme = URL_SCHEME_PATTERN.exec(absoluteUrl)?.[1]?.toLowerCase();
  if (scheme === undefined) {
   return compareUrlToPageOrigin(absoluteUrl, pageOrigin);
  }
  if (Object.hasOwn(FOREIGN_SCRIPT_SCHEMES, scheme)) {
   return 'foreign';
  }
  if (scheme !== 'http' && scheme !== 'https') {
   return 'unknown';
  }
  return compareUrlToPageOrigin(absoluteUrl, pageOrigin);
 }

 const withoutPosition = reference.replace(/:\d+:\d+$/u, '');
 if (withoutPosition.startsWith('/') && !withoutPosition.startsWith('//')) {
  return 'ours';
 }
 if (RELATIVE_SCRIPT_FRAME_PATTERN.test(reference)) {
  return 'ours';
 }
 return 'unknown';
}

interface StackVerdict {
 readonly origin: FrameOrigin;
 readonly evidence: string | undefined;
}

/**
 * Walk the frames of a stack and return the first attributable one.
 *
 * @param ignoreOwnFrames - Skip frames from our own origin. Used for stacks
 *   captured at handler entry, whose top frames are always the handler itself;
 *   the interesting evidence is what dispatched the event below them.
 */
function classifyStack(
 stack: string | undefined,
 ignoreOwnFrames: boolean,
): StackVerdict {
 if (stack === undefined) {
  return { origin: 'unknown', evidence: undefined };
 }
 const pageOrigin = globalThis.location.origin;
 for (const rawLine of stack.split('\n')) {
  const frame = rawLine.trim();
  if (frame.length === 0) {
   continue;
  }
  const origin = classifyReference(frame, pageOrigin);
  if (origin === 'unknown' || (origin === 'ours' && ignoreOwnFrames)) {
   continue;
  }
  return { origin, evidence: frame };
 }
 return { origin: 'unknown', evidence: undefined };
}

/**
 * The stack of an Error-like reason, without assuming a same-realm `Error`
 * (cross-realm errors and `DOMException` fail `instanceof Error`).
 */
function stackOfException(reason: unknown): string | undefined {
 if (typeof reason !== 'object' || reason === null || !('stack' in reason)) {
  return undefined;
 }
 const stack: unknown = reason.stack;
 return typeof stack === 'string' && stack.length > 0 ? stack : undefined;
}

const UNATTRIBUTABLE_RATIONALE =
 'no usable attribution evidence; treating as our own code (never hide our own errors)';

export interface ErrorEventAttributionInput {
 /** `ErrorEvent.filename`: the script the error was thrown from. */
 readonly filename: string;
 /** `ErrorEvent.error`: the thrown value, if the browser exposed it. */
 readonly error: unknown;
}

/**
 * Decide whether an `error` event came from the wiki's own code.
 *
 * `filename` is direct evidence of the throwing script; the thrown error's
 * stack is the fallback when the browser withheld or mangled the filename.
 */
export function attributeErrorEvent(
 input: ErrorEventAttributionInput,
): ErrorAttribution {
 const filename = input.filename.trim();
 if (filename.length > 0) {
  const filenameOrigin = classifyReference(filename, globalThis.location.origin);
  if (filenameOrigin !== 'unknown') {
   return {
    origin: filenameOrigin,
    rationale: `error event filename points at ${filenameOrigin === 'ours' ? 'our own' : 'a foreign'} script: ${filename}`,
   };
  }
 }

 const verdict = classifyStack(stackOfException(input.error), false);
 if (verdict.origin !== 'unknown' && verdict.evidence !== undefined) {
  return {
   origin: verdict.origin,
   rationale: `error stack originates at ${verdict.origin === 'ours' ? 'our own' : 'a foreign'} script: ${verdict.evidence}`,
  };
 }

 return { origin: 'ours', rationale: UNATTRIBUTABLE_RATIONALE };
}

export interface RejectionAttributionInput {
 /** `PromiseRejectionEvent.reason`. */
 readonly reason: unknown;
 /**
  * Stack captured at the top of the rejection handler. A rejection reason is
  * often not an `Error` (`undefined`, `{}`, a `DOMException`), so the handler
  * entry stack is the only evidence of who dispatched the event.
  */
 readonly handlerEntryStack: string | undefined;
}

/**
 * Decide whether an unhandled promise rejection came from the wiki's own code.
 *
 * The reason's own stack wins when it has one. Otherwise the handler entry
 * stack is inspected below the handler's own frames - that is where a foreign
 * script that called `dispatchEvent` shows up (the canonical #1501 case).
 */
export function attributeRejection(
 input: RejectionAttributionInput,
): ErrorAttribution {
 const reasonStack = stackOfException(input.reason);
 if (reasonStack !== undefined) {
  const verdict = classifyStack(reasonStack, false);
  if (verdict.origin !== 'unknown' && verdict.evidence !== undefined) {
   return {
    origin: verdict.origin,
    rationale: `rejection reason stack originates at ${verdict.origin === 'ours' ? 'our own' : 'a foreign'} script: ${verdict.evidence}`,
   };
  }
 }

 const entryVerdict = classifyStack(input.handlerEntryStack, true);
 if (entryVerdict.origin === 'foreign' && entryVerdict.evidence !== undefined) {
  return {
   origin: 'foreign',
   rationale: `rejection was dispatched from a foreign script: ${entryVerdict.evidence}`,
  };
 }

 return { origin: 'ours', rationale: UNATTRIBUTABLE_RATIONALE };
}

export interface ForeignErrorLogDetails {
 readonly handler: 'error' | 'unhandledrejection';
 readonly attribution: ErrorAttribution;
 readonly reason: unknown;
 readonly isTrusted: boolean;
 readonly hasPromise: boolean;
 readonly handlerEntryStack: string | undefined;
}

/**
 * Log an error that was attributed to foreign code. The page stays alive and
 * the browser's own console reporting is left intact (the handlers do not call
 * `preventDefault()` on this path).
 */
export function logForeignError(details: ForeignErrorLogDetails): void {
 const {
  handler,
  attribution,
  reason,
  isTrusted,
  hasPromise,
  handlerEntryStack,
 } = details;

 let reasonConstructor = 'none';
 if (typeof reason === 'object' && reason !== null && 'constructor' in reason) {
  const candidate: unknown = reason.constructor;
  if (typeof candidate === 'function') {
   const name: unknown = candidate.name;
   reasonConstructor =
    typeof name === 'string' && name.length > 0 ? name : 'anonymous';
  }
 }

 let reasonString: string;
 try {
  reasonString = String(reason);
 } catch {
  reasonString = '<unstringifiable>';
 }

 console.warn(
  `[global-error-handler] Ignoring ${handler} from foreign code; the page stays alive. ${attribution.rationale}`,
  {
   handler,
   attribution: attribution.origin,
   rationale: attribution.rationale,
   reasonType: typeof reason,
   reasonConstructor,
   reasonString,
   reasonStack: stackOfException(reason),
   handlerEntryStack,
   isTrusted,
   hasPromise,
   reason,
  },
 );
}
