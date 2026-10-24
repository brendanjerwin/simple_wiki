/**
 * Tests for wiki-artifact-host with fully stubbed Connect clients.
 *
 * Element instances are created manually (document.createElement) so the
 * tests can replace the private Connect client objects BEFORE
 * connectedCallback fires — no real network calls ever happen.
 */
import { expect } from '@open-wc/testing';
import { stub, type SinonStub } from 'sinon';
import { create } from '@bufbuild/protobuf';
import {
 GetArtifactResponseSchema,
 GetArtifactStateResponseSchema,
 MergeArtifactStateResponseSchema,
 WatchArtifactResponseSchema,
 WatchArtifactResponse_Kind,
 type GetArtifactResponse,
} from '../gen/api/v1/artifact_pb.js';
import {
 WikiArtifactHost,
} from './wiki-artifact-host.js';

function artifactResponse(source: string): GetArtifactResponse {
 return create(GetArtifactResponseSchema, {
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
 return create(GetArtifactStateResponseSchema, {
  state: state as never,
  stateVersionHash: hash,
 });
}

/** A watch stream that yields the initial snapshot and then blocks open. */
// eslint-disable-next-line require-yield
async function* openWatchStream(initial: { kind: WatchArtifactResponse_Kind; source: string }) {
 yield create(WatchArtifactResponseSchema, initial);
 await new Promise(() => { }); // keep connected until aborted
}

type StubbedHost = {
 el: WikiArtifactHost;
 getArtifact: SinonStub;
 getArtifactState: SinonStub;
 mergeArtifactState: SinonStub;
 clearArtifact: SinonStub;
 watchArtifact: SinonStub;
};

/**
 * Build a host with stubbed clients. Clients are swapped in before the
 * element is connected, so load()/watch() run against the stubs only.
 */
async function stubbedHost(programSource = 'root = Card("Hello", [t])\nt = Text("world")'): Promise<StubbedHost> {
 const getArtifact = stub().resolves(artifactResponse(programSource));
 const getArtifactState = stub().resolves(stateResponse({}, 'state-hash-1'));
 const mergeArtifactState = stub().resolves(
  create(MergeArtifactStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
 );
 const clearArtifact = stub().resolves({});
 const watchArtifact = stub().returns(openWatchStream({
  kind: WatchArtifactResponse_Kind.INITIAL,
  source: programSource,
 }));

 const el = document.createElement('wiki-artifact-host') as WikiArtifactHost;
 await customElements.whenDefined('wiki-artifact-host');
 if (!(el instanceof WikiArtifactHost)) {
  throw new Error('wiki-artifact-host did not upgrade — is the custom element registered?');
 }
 el.page = 'stub-page';
 const clients = el as unknown as {
  artifactClient: Record<string, unknown>;
  stateClient: Record<string, unknown>;
 };
 clients.artifactClient['getArtifact'] = getArtifact;
 clients.artifactClient['clearArtifact'] = clearArtifact;
 clients.artifactClient['watchArtifact'] = watchArtifact;
 clients.stateClient['getArtifactState'] = getArtifactState;
 clients.stateClient['mergeArtifactState'] = mergeArtifactState;

 document.body.appendChild(el);
 await el.updateComplete;
 return { el, getArtifact, getArtifactState, mergeArtifactState, clearArtifact, watchArtifact };
}

describe('WikiArtifactHost @ToAssistant channel', () => {
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

 it('delivers a chat message on the artifact page', () => {
  stubs.el.dispatchEvent(new CustomEvent('wiki-artifact-action', {
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
  stubs.el.dispatchEvent(new CustomEvent('wiki-artifact-action', {
   detail: { plan: { steps: [{ type: 'continue_conversation' as never, message: 'session done' }] } },
   bubbles: true,
   composed: true,
  }));
  expect(sendMessage).to.have.been.calledOnce;
  expect(sendMessage.firstCall.args[0].content).to.equal('session done');
 });
});

describe('WikiArtifactHost', () => {
 describe('when the page has an artifact', () => {
  let stubs: StubbedHost;

  beforeEach(async () => {
   stubs = await stubbedHost();
  });

  afterEach(() => {
   stubs.el.remove();
  });

  it('should fetch the artifact with the page attribute', () => {
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

 describe('when the remove button is clicked', () => {
  let stubs: StubbedHost | null = null;
  let removedEvent: Event | null = null;

  beforeEach(async () => {
   stubs = await stubbedHost();
   stubs?.el.addEventListener('wiki-artifact-removed', (event) => {
    removedEvent = event;
   });
   await stubs.el.updateComplete;
   const removeButton = stubs.el.shadowRoot?.querySelector<HTMLButtonElement>('.wiki-artifact-remove');
   expect(removeButton).to.exist;
   removeButton?.click();
   await stubs.el.updateComplete;
   await new Promise((resolve) => setTimeout(resolve, 10));
  });

  it('should call ClearArtifact with the artifact version hash', async () => {
   expect(stubs?.clearArtifact).to.have.been.calledOnce;
   const request = stubs?.clearArtifact.firstCall.args[0];
   expect(request?.page).to.equal('stub-page');
   expect(request?.expectedVersionHash).to.equal('hash-1');
  });

  it('should dispatch wiki-artifact-removed for the shell to reload plain markdown', () => {
   expect(removedEvent).to.exist;
  });
 });

 describe('when the page lacks an artifact', () => {
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
describe('WikiArtifactHost suspension recovery', () => {
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
  const getArtifact = stub().resolves(artifactResponse(program));
  const getArtifactState = stub().resolves(stateResponse({}, 'state-hash-1'));
  const mergeArtifactState = stub().resolves(
   create(MergeArtifactStateResponseSchema, { state: {} as never, stateVersionHash: 'state-hash-2' }),
  );
  const clearArtifact = stub().resolves({});

  // First watch call: yields INITIAL then dies (suspension). Second
  // call (retry): healthy open stream.
  let callCount = 0;
  const watchArtifact = stub().callsFake(() => {
   callCount += 1;
   if (callCount === 1) {
    return (async function*() {
     yield create(WatchArtifactResponseSchema, {
      kind: WatchArtifactResponse_Kind.INITIAL,
      source: program,
     });
     // Simulate the suspension: stream tears down with a network error.
     throw new Error('network connection lost');
    })();
   }
   return openWatchStream({ kind: WatchArtifactResponse_Kind.INITIAL, source: program });
  });

  const el = document.createElement('wiki-artifact-host') as WikiArtifactHost;
  el.page = 'stub-page';
  const clients = el as unknown as Record<string, Record<string, unknown>>;
  clients['artifactClient'] = { getArtifact, clearArtifact, watchArtifact };
  clients['stateClient'] = { getArtifactState, mergeArtifactState };
  document.body.appendChild(el);
  await el.updateComplete;

  // Initial render happened before the stream died.
  const textAfterFirst = () => el.shadowRoot?.textContent ?? '';
  await new Promise((resolve) => setTimeout(resolve, 20));
  expect(textAfterFirst()).to.include('Hello');

  // Suspended: stream dies → reconnecting banner (NOT the error screen).
  await new Promise((resolve) => setTimeout(resolve, 30));
  expect(el.shadowRoot?.querySelector('.reconnecting-note')).to.exist;
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
});