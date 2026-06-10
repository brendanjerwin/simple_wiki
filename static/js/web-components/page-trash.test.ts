import { expect, fixture, html, waitUntil } from '@open-wc/testing';
import { create } from '@bufbuild/protobuf';
import sinon from 'sinon';
import './page-trash.js';
import type { PageTrash } from './page-trash.js';
import { TimestampSchema } from '@bufbuild/protobuf/wkt';
import type {
  EmptyTrashResponse,
  ListTrashResponse,
  RestorePageResponse,
  TrashPageEntry,
} from '../gen/api/v1/page_management_pb.js';

function timestamp(seconds: bigint) {
  return create(TimestampSchema, { seconds, nanos: 0 });
}

function trashEntry(overrides: Partial<TrashPageEntry> = {}): TrashPageEntry {
  return {
    $typeName: 'api.v1.TrashPageEntry',
    trashId: 'trash-1',
    identifier: 'weekly_menu',
    title: 'Weekly Menu',
    deletedAt: timestamp(1_780_000_000n),
    deletedBy: 'user@example.com',
    purgesAt: timestamp(1_782_592_000n),
    daysRemaining: 30,
    ...overrides,
  };
}

function listResponse(pages: TrashPageEntry[]): ListTrashResponse {
  return {
    $typeName: 'api.v1.ListTrashResponse',
    pages,
  };
}

function restoreResponse(restoredIdentifier: string): RestorePageResponse {
  return {
    $typeName: 'api.v1.RestorePageResponse',
    success: true,
    error: '',
    restoredIdentifier,
  };
}

function emptyTrashResponse(purgedCount: number): EmptyTrashResponse {
  return {
    $typeName: 'api.v1.EmptyTrashResponse',
    success: true,
    error: '',
    purgedCount,
  };
}

describe('PageTrash', () => {
  let el: PageTrash;
  let client: {
    listTrash: sinon.SinonStub;
    restorePage: sinon.SinonStub;
    purgePage: sinon.SinonStub;
    emptyTrash: sinon.SinonStub;
  };
  let confirmStub: sinon.SinonStub;

  async function renderElement(): Promise<PageTrash> {
    const element = await fixture<PageTrash>(html`<page-trash .autoLoad=${false}></page-trash>`);
    element.client = client as unknown as PageTrash['client'];
    await element.refresh();
    await element.updateComplete;
    return element;
  }

  beforeEach(() => {
    client = {
      listTrash: sinon.stub().resolves(listResponse([trashEntry()])),
      restorePage: sinon.stub().resolves(restoreResponse('weekly_menu')),
      purgePage: sinon.stub().resolves({ $typeName: 'api.v1.PurgePageResponse', success: true }),
      emptyTrash: sinon.stub().resolves(emptyTrashResponse(1)),
    };
    confirmStub = sinon.stub(globalThis, 'confirm').returns(true);
  });

  afterEach(() => {
    sinon.restore();
  });

  describe('when trash loads successfully', () => {
    beforeEach(async () => {
      el = await renderElement();
    });

    it('should render the trash entry', () => {
      expect(el.shadowRoot?.textContent).to.contain('Weekly Menu');
      expect(el.shadowRoot?.textContent).to.contain('weekly_menu');
    });

    it('should render empty trash command', () => {
      expect(el.shadowRoot?.querySelector('.trash-header button')).to.exist;
    });
  });

  describe('when trash is empty', () => {
    beforeEach(async () => {
      client.listTrash.resolves(listResponse([]));
      el = await renderElement();
    });

    it('should render the empty state', () => {
      expect(el.shadowRoot?.textContent).to.contain('Trash is empty.');
    });

    it('should not render empty trash command', () => {
      expect(el.shadowRoot?.querySelector('.trash-header button')).to.not.exist;
    });
  });

  describe('when list fails', () => {
    beforeEach(async () => {
      client.listTrash.rejects(new Error('boom'));
      el = await renderElement();
    });

    it('should show an error display', () => {
      expect(el.shadowRoot?.querySelector('error-display')).to.exist;
    });
  });

  describe('when restoring a page', () => {
    beforeEach(async () => {
      el = await renderElement();
      const restoreButton = el.shadowRoot?.querySelector('tbody button');
      restoreButton?.dispatchEvent(new Event('click'));
      await waitUntil(() => client.restorePage.calledOnce);
      await el.updateComplete;
    });

    it('should call restore with the trash id', () => {
      expect(client.restorePage).to.have.been.calledOnce;
      expect(client.restorePage.firstCall.args[0].trashId).to.equal('trash-1');
    });

    it('should refresh trash after restore', () => {
      expect(client.listTrash).to.have.been.calledTwice;
    });
  });

  describe('when purging a page is confirmed', () => {
    beforeEach(async () => {
      el = await renderElement();
      const purgeButton = el.shadowRoot?.querySelectorAll('tbody button')[1];
      purgeButton?.dispatchEvent(new Event('click'));
      await waitUntil(() => client.purgePage.calledOnce);
      await el.updateComplete;
    });

    it('should ask for confirmation', () => {
      expect(confirmStub).to.have.been.calledWith('Permanently purge weekly_menu?');
    });

    it('should call purge with the trash id', () => {
      expect(client.purgePage).to.have.been.calledOnce;
      expect(client.purgePage.firstCall.args[0].trashId).to.equal('trash-1');
    });
  });

  describe('when purging a page is cancelled', () => {
    beforeEach(async () => {
      confirmStub.returns(false);
      el = await renderElement();
      const purgeButton = el.shadowRoot?.querySelectorAll('tbody button')[1];
      purgeButton?.dispatchEvent(new Event('click'));
      await el.updateComplete;
    });

    it('should not call purge', () => {
      expect(client.purgePage).not.to.have.been.called;
    });
  });

  describe('when emptying trash is confirmed', () => {
    beforeEach(async () => {
      el = await renderElement();
      const emptyButton = el.shadowRoot?.querySelector('.trash-header button');
      emptyButton?.dispatchEvent(new Event('click'));
      await waitUntil(() => client.emptyTrash.calledOnce);
      await el.updateComplete;
    });

    it('should call empty trash', () => {
      expect(client.emptyTrash).to.have.been.calledOnce;
    });
  });
});
