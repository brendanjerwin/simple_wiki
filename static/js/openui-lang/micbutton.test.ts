/**
 * Behavioural tests for `wiki-page-app-mic-button` (the MicButton component's
 * interactive element). A fake `SpeechRecognition` constructor stands in for
 * the browser API so the recognition pipeline — start, interim/final results,
 * error classes, end-of-session dispatch, abort semantics, hold vs tap mode and
 * the keyboard path — is exercised without a microphone.
 */
import { expect } from '@open-wc/testing';
import { fixture, html } from '@open-wc/testing';
import { PAGE_APP_ACTION_EVENT } from './library.js';
import type { PageAppActionEventDetail, WikiPageAppMicButton } from './library.js';

interface FakeResultSpec {
 transcript: string;
 isFinal: boolean;
}

/** Build a `SpeechRecognitionResultList`-shaped value from simple specs. */
function resultList(specs: FakeResultSpec[]): SpeechRecognitionResultList {
 const list: unknown[] = specs.map((spec) => {
  const alternatives: unknown[] = [{ transcript: spec.transcript, confidence: 1 }];
  Object.assign(alternatives, { isFinal: spec.isFinal, length: 1 });
  return alternatives;
 });
 Object.assign(list, { length: specs.length });
 return list as unknown as SpeechRecognitionResultList;
}

class FakeSpeechRecognition {
 static instances: FakeSpeechRecognition[] = [];

 continuous = false;
 interimResults = false;
 maxAlternatives = 1;
 lang = '';
 onresult: ((event: unknown) => void) | null = null;
 onerror: ((event: unknown) => void) | null = null;
 onend: (() => void) | null = null;

 startCount = 0;
 stopCount = 0;
 abortCount = 0;

 constructor() {
  FakeSpeechRecognition.instances.push(this);
 }

 start(): void {
  this.startCount += 1;
 }

 stop(): void {
  this.stopCount += 1;
 }

 abort(): void {
  this.abortCount += 1;
 }

 /** Simulate the browser delivering a result batch. */
 emitResults(specs: FakeResultSpec[], resultIndex = 0): void {
  this.onresult?.({ resultIndex, results: resultList(specs) });
 }

 emitError(error: string): void {
  this.onerror?.({ error });
 }

 /** Simulate the browser ending the session (always fires after stop/abort). */
 endSession(): void {
  this.onend?.();
 }
}

function latestRecognition(): FakeSpeechRecognition {
 const rec = FakeSpeechRecognition.instances.at(-1);
 if (rec == null) throw new Error('expected the element to construct a recognition instance');
 return rec;
}

const realGlobals = {
 speechRecognition: (globalThis as Record<string, unknown>)['SpeechRecognition'],
 webkitSpeechRecognition: (globalThis as Record<string, unknown>)['webkitSpeechRecognition'],
};

function installFakeSpeechRecognition(): void {
 const g = globalThis as Record<string, unknown>;
 g['SpeechRecognition'] = FakeSpeechRecognition;
 delete g['webkitSpeechRecognition'];
}

function restoreGlobals(): void {
 const g = globalThis as Record<string, unknown>;
 if (realGlobals.speechRecognition === undefined) delete g['SpeechRecognition'];
 else g['SpeechRecognition'] = realGlobals.speechRecognition;
 if (realGlobals.webkitSpeechRecognition === undefined) delete g['webkitSpeechRecognition'];
 else g['webkitSpeechRecognition'] = realGlobals.webkitSpeechRecognition;
}

/** Collect every `wiki-page-app-action` the element dispatches. */
function captureActions(el: WikiPageAppMicButton): PageAppActionEventDetail[] {
 const captured: PageAppActionEventDetail[] = [];
 el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
  captured.push((event as CustomEvent<PageAppActionEventDetail>).detail);
 });
 return captured;
}

describe('WikiPageAppMicButton (supported browser)', () => {
 beforeEach(() => {
  FakeSpeechRecognition.instances = [];
  installFakeSpeechRecognition();
 });

 afterEach(() => {
  restoreGlobals();
 });

 async function mount(props: { label?: string; mode?: 'hold' | 'tap'; template?: string } = {}): Promise<WikiPageAppMicButton> {
  const el = await fixture<WikiPageAppMicButton>(html`
   <wiki-page-app-mic-button
    .label=${props.label ?? 'Speak'}
    .template=${props.template}
    .mode=${props.mode ?? 'hold'}
   ></wiki-page-app-mic-button>`);
  await el.updateComplete;
  return el;
 }

 function button(el: WikiPageAppMicButton): HTMLButtonElement {
  const btn = el.shadowRoot?.querySelector('button');
  if (btn == null) throw new Error('expected the element to render a button');
  return btn as HTMLButtonElement;
 }

 function press(btn: HTMLElement, type: 'pointerdown' | 'pointerup' | 'pointercancel' | 'pointerleave'): void {
  btn.dispatchEvent(new PointerEvent(type, { bubbles: true, composed: true, pointerId: 1 }));
 }

 function key(btn: HTMLElement, type: 'keydown' | 'keyup', code: string): void {
  btn.dispatchEvent(new KeyboardEvent(type, { bubbles: true, composed: true, code }));
 }

 describe('when connected in a browser that supports speech recognition', () => {
  it('should render an enabled button', async () => {
   const el = await mount();
   const btn = button(el);
   expect(btn.disabled).to.equal(false);
   expect(btn.getAttribute('aria-label')).to.equal('Speak');
   expect(btn.getAttribute('aria-pressed')).to.equal('false');
   expect(el.shadowRoot?.querySelector('.mic-unsupported-label')).to.equal(null);
  });
 });

 describe('hold mode', () => {
  it('should start recognition synchronously on pointerdown', async () => {
   const el = await mount();
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   expect(rec.startCount).to.equal(1);
   await el.updateComplete;
   expect(button(el).getAttribute('data-listening')).to.equal('true');
  });

  it('should show the interim transcript while listening', async () => {
   const el = await mount();
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'hello', isFinal: false }]);
   await el.updateComplete;
   expect(el.shadowRoot?.querySelector('.mic-interim')?.textContent?.trim()).to.equal('hello');
  });

  it('should dispatch the raw transcript on pointerup then end-of-session', async () => {
   const el = await mount();
   const actions = captureActions(el);
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'add oat milk', isFinal: true }]);
   press(button(el), 'pointerup');
   expect(rec.stopCount).to.equal(1);
   rec.endSession();
   await el.updateComplete;
   expect(actions).to.have.length(1);
   expect(actions[0]?.plan?.steps[0]).to.deep.equal({ type: 'continue_conversation', message: 'add oat milk' });
   expect(el.shadowRoot?.querySelector('.mic-interim')).to.equal(null);
  });

  it('should interpolate the {{transcript}} template', async () => {
   const el = await mount({ template: 'Price this: {{transcript}} (again: {{transcript}})' });
   const actions = captureActions(el);
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'a standing desk', isFinal: true }]);
   press(button(el), 'pointerup');
   rec.endSession();
   expect(actions[0]?.plan?.steps[0]).to.deep.equal({
    type: 'continue_conversation',
    message: 'Price this: a standing desk (again: a standing desk)',
   });
  });

  it('should not dispatch when the transcript is blank', async () => {
   const el = await mount();
   const actions = captureActions(el);
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: '   ', isFinal: true }]);
   press(button(el), 'pointerup');
   rec.endSession();
   expect(actions).to.have.length(0);
  });

  it('should abort without dispatching on pointercancel', async () => {
   const el = await mount();
   const actions = captureActions(el);
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'never mind', isFinal: true }]);
   press(button(el), 'pointercancel');
   expect(rec.stopCount).to.equal(1);
   rec.endSession();
   await el.updateComplete;
   expect(actions).to.have.length(0);
  });

  it('should abort without dispatching on Escape', async () => {
   const el = await mount();
   const actions = captureActions(el);
   const btn = button(el);
   press(btn, 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'scratch that', isFinal: true }]);
   key(btn, 'keydown', 'Escape');
   expect(rec.stopCount).to.equal(1);
   rec.endSession();
   await el.updateComplete;
   expect(actions).to.have.length(0);
  });

  it('should send on pointerleave (hold released off the button)', async () => {
   const el = await mount();
   const actions = captureActions(el);
   const btn = button(el);
   press(btn, 'pointerdown');
   const rec = latestRecognition();
   rec.emitResults([{ transcript: 'released off target', isFinal: true }]);
   press(btn, 'pointerleave');
   rec.endSession();
   expect(actions).to.have.length(1);
  });
 });

 describe('tap mode', () => {
  it('should start on first click and stop on second, then dispatch', async () => {
   const el = await mount({ mode: 'tap' });
   const actions = captureActions(el);
   const btn = button(el);
   btn.click();
   const rec = latestRecognition();
   expect(rec.startCount).to.equal(1);
   expect(rec.continuous).to.equal(true);
   rec.emitResults([{ transcript: 'set a reminder', isFinal: true }]);
   btn.click();
   expect(rec.stopCount).to.equal(1);
   rec.endSession();
   expect(actions[0]?.plan?.steps[0]).to.deep.equal({ type: 'continue_conversation', message: 'set a reminder' });
  });

  it('should ignore pointerdown in tap mode', async () => {
   const el = await mount({ mode: 'tap' });
   press(button(el), 'pointerdown');
   expect(FakeSpeechRecognition.instances).to.have.length(0);
  });
 });

 describe('keyboard hold path', () => {
  it('should start on Space keydown and stop on keyup', async () => {
   const el = await mount();
   const actions = captureActions(el);
   const btn = button(el);
   key(btn, 'keydown', 'Space');
   const rec = latestRecognition();
   expect(rec.startCount).to.equal(1);
   rec.emitResults([{ transcript: 'typed with space held', isFinal: true }]);
   key(btn, 'keyup', 'Space');
   expect(rec.stopCount).to.equal(1);
   rec.endSession();
   expect(actions).to.have.length(1);
  });

  it('should not restart while the key is still held', async () => {
   const el = await mount();
   const btn = button(el);
   key(btn, 'keydown', 'Space');
   key(btn, 'keydown', 'Space');
   expect(FakeSpeechRecognition.instances).to.have.length(1);
  });
 });

 describe('error handling', () => {
  it('should latch permission denial, disable the button and never retry', async () => {
   const el = await mount();
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitError('not-allowed');
   rec.endSession();
   await el.updateComplete;
   const btn = button(el);
   expect(btn.disabled).to.equal(true);
   expect(el.shadowRoot?.querySelector('.mic-status')?.textContent).to.include('Microphone access was blocked');
   press(btn, 'pointerdown');
   expect(FakeSpeechRecognition.instances).to.have.length(1);
  });

  it('should show a retry hint for no-speech', async () => {
   const el = await mount();
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitError('no-speech');
   rec.endSession();
   await el.updateComplete;
   expect(el.shadowRoot?.querySelector('.mic-status')?.textContent).to.include("Didn't catch that");
  });

  it('should show a connection error for network failures', async () => {
   const el = await mount();
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   rec.emitError('network');
   rec.endSession();
   await el.updateComplete;
   const status = el.shadowRoot?.querySelector('.mic-status');
   expect(status?.textContent).to.include('needs a network connection');
   expect(status?.getAttribute('data-kind')).to.equal('error');
  });
 });

 describe('component props', () => {
  it('should forward lang to the recogniser and honour interim=false', async () => {
   const el = await fixture<WikiPageAppMicButton>(html`
    <wiki-page-app-mic-button .label=${'Speak'} .recognitionLang=${'fr-FR'} .interim=${false}></wiki-page-app-mic-button>`);
   await el.updateComplete;
   press(button(el), 'pointerdown');
   const rec = latestRecognition();
   expect(rec.lang).to.equal('fr-FR');
   expect(rec.interimResults).to.equal(false);
  });
 });
});
