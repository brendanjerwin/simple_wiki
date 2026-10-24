/**
 * Tests for wiki-page-app-host with fully stubbed Connect clients.
 *
 * Element instances are created manually (document.createElement) so the
 * tests can replace the private Connect client objects BEFORE
 * connectedCallback fires — no real network calls ever happen.
 */
import { expect } from '@open-wc/testing';
import { stub, type SinonStub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import { ConnectError, Code } from '@connectrpc/connect';
import {
 GetPageAppResponseSchema,
 GetPageAppStateResponseSchema,
 MergePageAppStateResponseSchema,
 WatchPageAppResponseSchema,
 WatchPageAppResponse_Kind,
 type GetPageAppResponse,
} from '../gen/api/v1/page_app_pb.js';
import {
 WikiPageAppHost,
} from './wiki-page-app-host.js';
import type { PageAppActionEventDetail } from '../openui-lang/library.js';

function pageAppResponse(source: string): GetPageAppResponse {
 return create(GetPageAppResponseSchema, {
  source,
  lang: 'openui-lang',
  version: BigInt(1),
  versionHash: 'hash-1',
  library: 'wiki-core@1',
  updatedBy: 'test',
  automated: false,
 });
}

function stateResponse(state: Record<string, unknown>, hash: string) {
 return create(GetPageAppStateResponseSchema, {
  state: state as never,
  stateVersionHash: hash,
 });
}

/** A watch stream that yields the initial snapshot and then blocks open. */
// eslint-disable-next-line require-yield
async function* openWatchStream(initial: { kind: WatchPageAppResponse_Kind; source: string }) {
 yield create(WatchPageAppResponseSchema, initial);
 await new Promise(() => { }); // keep connected until aborted
}

type StubbedHost = {
 el: WikiPageAppHost;
 getArtifact: SinonStub;
 getPageAppState: SinonStub;
 mergePageAppState: SinonStub;
 clearArtifact: SinonStub;
 watchArtifact: SinonStub;
};

/**
 * Build a host with stubbed clients. Clients are swapped in before the
 * element is connected, so load()/watch() run against the stubs only.
 */
async function stubbedHost(programSource = 'root = Card("Hello", [t])\nt = Text("world")'): Promise<StubbedHost> {
 const getArtifact = stub().resolves(pageAppResponse(programSource));
 const getPageAppState = stub().resolves(stateResponse({}, 'state-hash-1'));
 const mergePageAppState = stub().resolves(
  create(MergePageAppStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
 );
 const clearArtifact = stub().resolves({});
 const watchArtifact = stub().returns(openWatchStream({
  kind: WatchPageAppResponse_Kind.INITIAL,
  source: programSource,
 }));

 const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
 await customElements.whenDefined('wiki-page-app-host');
 if (!(el instanceof WikiPageAppHost)) {
  throw new Error('wiki-page-app-host did not upgrade — is the custom element registered?');
 }
 el.page = 'stub-page';
 const clients = el as unknown as {
  pageAppClient: Record<string, unknown>;
  stateClient: Record<string, unknown>;
 };
 clients.pageAppClient['getPageApp'] = getArtifact;
 clients.pageAppClient['clearPageApp'] = clearArtifact;
 clients.pageAppClient['watchPageApp'] = watchArtifact;
 clients.stateClient['getPageAppState'] = getPageAppState;
 clients.stateClient['mergePageAppState'] = mergePageAppState;

 document.body.appendChild(el);
 await el.updateComplete;
 return { el, getArtifact, getPageAppState, mergePageAppState, clearArtifact, watchArtifact };
}

describe('WikiPageAppHost @ToAssistant channel', () => {
 let sendMessage: SinonStub;
 let stubs: StubbedHost;

 beforeEach(async () => {
  stubs = await stubbedHost();
  // Swap in a chat client stub; @ToAssistant fires ChatService SendMessage.
  sendMessage = stub().resolves({});
  const clients = stubs.el as unknown as { chatClient: Record<string, SinonStub> };
  clients.chatClient['sendMessage'] = sendMessage;
  await stubs.el.updateComplete;
 });

 it('delivers a chat message on the page app page', () => {
  stubs.el.dispatchEvent(new CustomEvent('wiki-page-app-action', {
   detail: { plan: { steps: [{ type: 'continue_conversation' as never, message: 'increment completed sessions' }] } },
   bubbles: true,
   composed: true,
  }));
  expect(sendMessage).to.have.been.calledOnce;
  const request = sendMessage.firstCall.args[0];
  expect(request.page).to.equal('stub-page');
  expect(request.content).to.equal('increment completed sessions');
 });

 it('a button with no action sends its label (auto-ToAssistant)', () => {
  stubs.el.dispatchEvent(new CustomEvent('wiki-page-app-action', {
   detail: { plan: { steps: [{ type: 'continue_conversation' as never, message: 'session done' }] } },
   bubbles: true,
   composed: true,
  }));
  expect(sendMessage).to.have.been.calledOnce;
  expect(sendMessage.firstCall.args[0].content).to.equal('session done');
 });
});

describe('WikiPageAppHost', () => {
 describe('when the page has a page app', () => {
  let stubs: StubbedHost;

  beforeEach(async () => {
   stubs = await stubbedHost();
  });

  afterEach(() => {
   stubs.el.remove();
  });

  it('should fetch the page app with the page attribute', () => {
   expect(stubs.getArtifact).to.have.been.calledOnce;
   const request = stubs.getArtifact.firstCall.args[0];
   expect(request.page).to.equal('stub-page');
  });

  it('should render the program into the shadow DOM', async () => {
   await new Promise((resolve) => setTimeout(resolve, 20));
   await stubs.el.updateComplete;
   expect(stubs.el.shadowRoot?.innerHTML ?? '').to.include('Hello');
   expect(stubs.el.shadowRoot?.innerHTML ?? '').to.include('world');
  });

  it('should show the remove-the-app affordance', async () => {
   await stubs.el.updateComplete;
   expect(stubs.el.shadowRoot?.innerHTML ?? '').to.include('Remove the app');
  });
 });

 describe('when an Input onChange @Set fires', () => {
  let stubs: StubbedHost;
  let mergeCalls: Record<string, unknown>[];

  beforeEach(async () => {
   stubs = await stubbedHost(
    '$name = ""\nroot = Card("T", [Input("your name", $name), Text("hi " + $name)])',
   );
   mergeCalls = [];
   const clients = stubs.el as unknown as { stateClient: Record<string, unknown> };
   clients.stateClient['mergePageAppState'] = stub().callsFake((req: { state: Record<string, unknown> }) => {
    mergeCalls.push({ ...req.state });
    return Promise.resolve(
     create(MergePageAppStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
    );
   });

   // Simulate typing: the Input control dispatches the onChange plan with
   // the control's value as detail.value.
   stubs.el.dispatchEvent(new CustomEvent('wiki-page-app-action', {
    detail: {
     plan: { steps: [{ type: 'set' as never, target: '$name', valueAST: { k: 'StateRef', n: '$value' } }] },
     value: 'Ada',
    },
    bubbles: true,
    composed: true,
   }));
   await stubs.el.updateComplete;
   await new Promise((resolve) => setTimeout(resolve, 10));
  });

  it('should write the typed value into the store as the target variable', () => {
   expect(mergeCalls[0]?.['$name']).to.equal('Ada');
  });

  it('should flush the merged snapshot to PageAppStateService', () => {
   expect(mergeCalls).to.have.lengthOf(1);
  });
 });

 describe('when a rendered Countdown completes', () => {
  let stubs: StubbedHost;
  let mergeCalls: Record<string, unknown>[];

  beforeEach(async () => {
   // Program exactly as authored (the @Set refers to $holds; the element's
   // completion dispatch must land $holds + 1 in the host store).
   stubs = await stubbedHost(
    '$holds = 0\nroot = Card("T", [c])\nc = Countdown(30, "hold", @Set($holds, $holds + 1))',
   );
   mergeCalls = [];
   const clients = stubs.el as unknown as { stateClient: Record<string, unknown> };
   clients.stateClient['mergePageAppState'] = stub().callsFake((req: { state: Record<string, unknown> }) => {
    mergeCalls.push({ ...req.state });
    return Promise.resolve(
     create(MergePageAppStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
    );
   });
   await stubs.el.updateComplete;
   await new Promise((resolve) => setTimeout(resolve, 20));
   await stubs.el.updateComplete;
  });

  it('should execute the completed plan against the host store and flush it', async () => {
   // Find the countdown rendered INSIDE the host's shadow root and fire
   // its completion dispatch — composed so it crosses the shadow
   // boundary to the host listener, exactly like a real tick.
   const countdown = stubs.el.shadowRoot?.querySelector('wiki-page-app-countdown');
   expect(countdown, 'program must render a Countdown in the host shadow root').to.exist;

   const completedDetail: PageAppActionEventDetail | undefined = { plan: { steps: [{ type: 'set' as never, target: '$holds', valueAST: { k: 'BinOp', op: '+', left: { k: 'StateRef', n: '$holds' }, right: { k: 'Num', v: 1 } } }] } };
   countdown?.dispatchEvent(new CustomEvent('wiki-page-app-action', {
    detail: completedDetail satisfies PageAppActionEventDetail,
    bubbles: true,
    composed: true,
   }));
   await stubs.el.updateComplete;
   await new Promise((resolve) => setTimeout(resolve, 10));

   expect(mergeCalls).to.have.lengthOf(1, 'completed countdown must flush the store snapshot');
   expect(mergeCalls[0]?.['$holds']).to.equal(1, '@Set($holds, $holds + 1) must land in the flushed snapshot');
  });
 });

 describe('when the remove button is clicked', () => {
  let stubs: StubbedHost | null = null;
  let removedEvent: Event | null = null;

  beforeEach(async () => {
   stubs = await stubbedHost();
   stubs?.el.addEventListener('wiki-page-app-removed', (event) => {
    removedEvent = event;
   });
   await stubs.el.updateComplete;
   const removeButton = stubs.el.shadowRoot?.querySelector<HTMLButtonElement>('.wiki-page-app-remove');
   expect(removeButton).to.exist;
   removeButton?.click();
   await stubs.el.updateComplete;
   await new Promise((resolve) => setTimeout(resolve, 10));
  });

  it('should call ClearPageApp with the page app version hash', async () => {
   expect(stubs?.clearArtifact).to.have.been.calledOnce;
   const request = stubs?.clearArtifact.firstCall.args[0];
   expect(request?.page).to.equal('stub-page');
   expect(request?.expectedVersionHash).to.equal('hash-1');
  });

  it('should dispatch wiki-page-app-removed for the shell to reload plain markdown', () => {
   expect(removedEvent).to.exist;
  });
 });

 describe('when the page lacks a page app', () => {
  let stubs: StubbedHost;

  beforeEach(async () => {
   stubs = await stubbedHost('');
   await stubs.el.updateComplete;
  });

  it('should render an empty shell', () => {
   expect(stubs.el.shadowRoot?.innerHTML ?? '').to.not.include('Remove the app');
  });
 });
});

// Suspension/reconnect recovery (issue #1272 user feedback): a killed
// watch stream — exactly what happens when a browser suspends the tab —
// must show a non-blocking "reconnecting" banner and recover WITHOUT a
// manual refresh.
describe('WikiPageAppHost suspension recovery', () => {
 // Deterministic backoff: the full-jitter ladder is randomized, so pin
 // the timer machinery by replacing Math.random during these tests.
 let randomStub: SinonStub;

 beforeEach(() => {
  randomStub = stub(Math, 'random').returns(0.999);
 });

 afterEach(() => {
  randomStub.restore();
 });

 it('shows the reconnecting banner and recovers the app without refresh', async () => {
  const program = 'root = Card("Hello", [t])\nt = Text("world")';
  const getArtifact = stub().resolves(pageAppResponse(program));
  const getPageAppState = stub().resolves(stateResponse({}, 'state-hash-1'));
  const mergePageAppState = stub().resolves(
   create(MergePageAppStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
  );
  const clearArtifact = stub().resolves({});

  // First watch call: yields INITIAL then dies (suspension). Second
  // call (retry): healthy open stream.
  let callCount = 0;
  const watchArtifact = stub().callsFake(() => {
   callCount += 1;
   if (callCount === 1) {
    return (async function*() {
     yield create(WatchPageAppResponseSchema, {
      kind: WatchPageAppResponse_Kind.INITIAL,
      source: program,
     });
     // Simulate the suspension: stream tears down with a network error.
     throw new Error('network connection lost');
    })();
   }
   return openWatchStream({ kind: WatchPageAppResponse_Kind.INITIAL, source: program });
  });

  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: getArtifact, clearPageApp: clearArtifact, watchPageApp: watchArtifact };
  clients['stateClient'] = { getPageAppState, mergePageAppState };
  document.body.appendChild(el);
  await el.updateComplete;

  // Initial render happened before the stream died.
  const textAfterFirst = () => el.shadowRoot?.textContent ?? '';
  await new Promise((resolve) => setTimeout(resolve, 20));
  expect(textAfterFirst()).to.include('Hello');

  // Suspended: stream dies → reconnecting banner (NOT the error screen).
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(el.shadowRoot?.querySelector('error-display')).to.not.exist;

  // Recovery ladder fires (first backoff = min(cap, 1000 * 2^0) with
  // jitter 0.999 → ~1s). Wait for the retried load.
  await new Promise((resolve) => setTimeout(resolve, 1300));
  await el.updateComplete;

  // Watch was called twice: original + recovery attempt.
  expect(callCount).to.equal(2);
  // App recovered: hydrated again, banner gone, no error screen.
  expect(el.shadowRoot?.textContent ?? '').to.include('Hello');
  expect(el.shadowRoot?.querySelector('.reconnecting-note')).to.not.exist;
  expect(el.shadowRoot?.querySelector('error-display')).to.not.exist;
  el.remove();
 });

 it('recovers on visibility return without waiting the backoff', async () => {
  // Shrink the ladder for deterministic, fast timings.
  const HostClass = WikiPageAppHost as unknown as {
   RETRY_BASE_MS: number;
   RETRY_MAX_ATTEMPTS: number;
   RETRY_MAX_MS: number;
  };
  const saved = { BASE: HostClass.RETRY_BASE_MS, MAX: HostClass.RETRY_MAX_ATTEMPTS, CAP: HostClass.RETRY_MAX_MS };
  HostClass.RETRY_BASE_MS = 200;
  HostClass.RETRY_MAX_ATTEMPTS = 5;
  HostClass.RETRY_MAX_MS = 400;

  const program = 'root = Card("Hello", [t])\nt = Text("world")';
  const getArtifact = stub().resolves(pageAppResponse(program));
  const getPageAppState = stub().resolves(stateResponse({}, 'state-hash-1'));
  // Watch dies on the FIRST call only (suspension); retries healthy.
  let watchCalls = 0;
  const watchArtifact = stub().callsFake(() => {
   watchCalls += 1;
   if (watchCalls === 1) {
    return (async function*() {
     yield create(WatchPageAppResponseSchema, { kind: WatchPageAppResponse_Kind.INITIAL, source: program });
     throw new Error('network connection lost');
    })();
   }
   return openWatchStream({ kind: WatchPageAppResponse_Kind.INITIAL, source: program });
  });
  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: getArtifact, clearPageApp: stub(), watchPageApp: watchArtifact };
  clients['stateClient'] = { getPageAppState, mergePageAppState: stub() };
  document.body.appendChild(el);

  // Wait for the reconnecting state (stream died, ladder running).
  const waitForReconnecting = async () => {
   const deadline = Date.now() + 3000;
   while (Date.now() < deadline) {
    await el.updateComplete;
    if (el.shadowRoot?.querySelector('.reconnecting-note')) return;
    await new Promise((resolve) => setTimeout(resolve, 100));
   }
  };
  await waitForReconnecting();

  // Mid-ladder: reconnecting banner is showing. Tab becomes visible →
  // immediate recovery instead of waiting the jitter delay.
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  document.dispatchEvent(new Event('visibilitychange'));

  const deadline3 = Date.now() + 5000;
  let visibleRecovered = false;
  while (Date.now() < deadline3 && !visibleRecovered) {
   await el.updateComplete;
   visibleRecovered = el.shadowRoot?.textContent?.includes('Hello') === true
    && !el.shadowRoot?.querySelector('.reconnecting-note');
   if (!visibleRecovered) await new Promise((resolve) => setTimeout(resolve, 200));
  }
  expect(visibleRecovered).to.be.true;
  expect(watchCalls).to.be.greaterThan(1);
  el.remove();
  HostClass.RETRY_BASE_MS = saved.BASE;
  HostClass.RETRY_MAX_ATTEMPTS = saved.MAX;
  HostClass.RETRY_MAX_MS = saved.CAP;
 });
});
// Issue #1283 bug 1 regression: load() detected the terminal NotFound case
// by matching the error *message* for the substring 'NotFound'. connect-web
// formats Go codes.NotFound as '[not_found] …' (lowercase), so the branch
// never matched: a page with no page app walked the whole retry ladder and
// surfaced a bogus 'failed after N retries' error. Terminal detection must
// use the shared isTerminal() helper (code-based), exactly like
// hydrateState and watch.
describe('WikiPageAppHost terminal NotFound on load', () => {
 it('should not schedule a retry when GetPageApp rejects with NotFound', async () => {
  const notFound = new ConnectError('no page app: stub-page', Code.NotFound);
  const getArtifact = stub().rejects(notFound);
  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: getArtifact, clearPageApp: stub(), watchPageApp: stub() };
  clients['stateClient'] = { getPageAppState: stub().resolves(stateResponse({}, 'state-hash-1')), mergePageAppState: stub() };
  // Replace the retry ladder with a counting spy: no timers are armed, so
  // the test needs no real-time waits — a buggy scheduleRetry call is
  // synchronously observable through the async load() catch path.
  let ladderRuns = 0;
  (el as unknown as { scheduleRetry: () => void }).scheduleRetry = () => { ladderRuns += 1; };
  document.body.appendChild(el);
  // Double updateComplete: one settle for the initial render; the second
  // pumps the async load() catch path so a buggy scheduleRetry call has
  // already happened before the assertion.
  await el.updateComplete;
  await el.updateComplete;

  expect(getArtifact.callCount).to.equal(1);
  expect(ladderRuns).to.equal(0);
  // Terminal NotFound renders the app-less state, not a reconnecting
  // banner or a retry-ladder error screen. Boolean-shaped so the
  // assertion message stays a primitive (WTR serializes failures with
  // structuredClone — a sinon stub in `actual` hangs the session).
  const shadow = el.shadowRoot;
  expect(shadow?.querySelector('.reconnecting-note') == null).to.equal(true);
  expect(shadow?.querySelector('error-display') == null).to.equal(true);
  expect((shadow?.textContent ?? '').includes('retries')).to.equal(false);
  el.remove();
 });

 it('should surface non-NotFound terminal errors instead of retrying or parking on the busy shell', async () => {
  // Control for the terminal branch's other codes: a PermissionDenied
  // can't be fixed by retrying — it must show the actual error, not the
  // reconnecting banner or an indefinite busy placeholder.
  const denied = new ConnectError('not allowed', Code.PermissionDenied);
  const getArtifact = stub().rejects(denied);
  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: getArtifact, clearPageApp: stub(), watchPageApp: stub() };
  clients['stateClient'] = { getPageAppState: stub().resolves(stateResponse({}, 'h')), mergePageAppState: stub() };
  let ladderRuns = 0;
  (el as unknown as { scheduleRetry: () => void }).scheduleRetry = () => { ladderRuns += 1; };
  document.body.appendChild(el);
  await el.updateComplete;
  await el.updateComplete;

  expect(getArtifact.callCount).to.equal(1);
  expect(ladderRuns).to.equal(0);
  const shadow = el.shadowRoot;
  expect(shadow?.querySelector('.reconnecting-note') == null).to.equal(true);
  expect(shadow?.querySelector('error-display') != null).to.equal(true);
  el.remove();
 });

 it('should still schedule a retry on a transient non-connect error', async () => {
  // Control guarding the terminal branch against over-matching: plain
  // Errors (not ConnectErrors) must still enter the retry ladder.
  const getArtifact = stub().rejects(new Error('network connection lost'));
  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: getArtifact, clearPageApp: stub(), watchPageApp: stub() };
  clients['stateClient'] = { getPageAppState: stub().resolves(stateResponse({}, 'h')), mergePageAppState: stub() };
  let ladderRuns = 0;
  (el as unknown as { scheduleRetry: () => void }).scheduleRetry = () => { ladderRuns += 1; };
  document.body.appendChild(el);
  await el.updateComplete;
  await el.updateComplete;

  expect(getArtifact.callCount).to.equal(1);
  expect(ladderRuns).to.equal(1);
  el.remove();
 });
});

// Issue #1283 bug 2 regression: toolProvider() resolved `page` from the
// program-supplied args (argPage(args, this.page)) for EVERY allowed RPC,
// including the three PageAppStateService methods. A Lang program could
// declare Mutation("MergePageAppState", { page: "other-page", … }) and
// read/merge/delete ANOTHER page's app state. Page app state is
// own-page-only: the host must pin all three state RPCs to its own page
// and ignore args.page entirely.
describe('WikiPageAppHost toolProvider state page pinning', () => {
 // toolProvider() is private on the host; the test seam is an unchecked
 // cast to a named local shape (same pattern as the client swaps in
 // stubbedHost above).
 type ToolProviderHost = { toolProvider(): { callTool(t: string, a: Record<string, unknown>): Promise<unknown> } };
 type StateClient = Record<string, SinonStub>;

 function hostWithStateClient(stateClient: StateClient, page = 'my-page'): { provider: ToolProviderHost; el: WikiPageAppHost } {
  const el = document.createElement('wiki-page-app-host') as WikiPageAppHost;
  el.page = page;
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['pageAppClient'] = { getPageApp: stub(), clearPageApp: stub(), watchPageApp: stub() };
  clients['stateClient'] = stateClient;
  // No connection needed: toolProvider() is a plain method over the
  // swapped-in client stubs.
  return { provider: el as unknown as ToolProviderHost, el };
 }

 let stateClient: StateClient;

 beforeEach(() => {
  stateClient = {
   getPageAppState: stub().resolves(stateResponse({}, 'hash-a')),
   mergePageAppState: stub().resolves(create(MergePageAppStateResponseSchema, { state: {} as never, stateVersionHash: 'hash-b' })),
  };
 });

 it('pins GetPageAppState to the host page even when args name another page', async () => {
  const { provider, el } = hostWithStateClient(stateClient);
  const { callTool } = provider.toolProvider();
  await callTool('GetPageAppState', { page: 'other-page' });
  expect(stateClient['getPageAppState']?.callCount).to.equal(1);
  expect((stateClient['getPageAppState']?.firstCall.args[0] as { page: string } | undefined)?.page).to.equal('my-page');
  el.remove();
 });

 it('pins MergePageAppState to the host page even when args name another page', async () => {
  const { provider, el } = hostWithStateClient(stateClient);
  const { callTool } = provider.toolProvider();
  await callTool('MergePageAppState', { page: 'other-page', state: { counter: 1 } });
  expect(stateClient['mergePageAppState']?.callCount).to.equal(1);
  expect((stateClient['mergePageAppState']?.firstCall.args[0] as { page: string } | undefined)?.page).to.equal('my-page');
  el.remove();
 });

 it('pins DeletePageAppState to the host page even when args name another page', async () => {
  stateClient['deletePageAppState'] = stub().resolves({});
  const { provider, el } = hostWithStateClient(stateClient);
  const { callTool } = provider.toolProvider();
  await callTool('DeletePageAppState', { page: 'other-page', stateExpectedVersionHash: 'h' });
  expect(stateClient['deletePageAppState']?.callCount).to.equal(1);
  expect((stateClient['deletePageAppState']?.firstCall.args[0] as { page: string } | undefined)?.page).to.equal('my-page');
  el.remove();
 });
});
