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
 return { el, getArtifact, getArtifactState, mergeArtifactState, clearArtifact };
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