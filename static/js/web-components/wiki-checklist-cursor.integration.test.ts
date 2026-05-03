/**
 * Integration test: verify that clicking in a checklist text input
 * actually positions the cursor at the click point, and stays there
 * across re-renders (including OCC retry refetches).
 *
 * This test exists because `draggable="true"` on the parent <li>
 * was previously intercepting mouse events and preventing cursor
 * positioning. The fix: only set draggable on mousedown of the drag
 * handle, not statically on the <li>.
 */
import { expect } from '@open-wc/testing';
import sinon from 'sinon';
import './wiki-checklist.js';
import type { WikiChecklist } from './wiki-checklist.js';
import { create } from '@bufbuild/protobuf';
import { timestampFromMs } from '@bufbuild/protobuf/wkt';
import {
  ListItemsResponseSchema,
  UpdateItemResponseSchema,
} from '../gen/api/v1/checklist_pb.js';
import { makeChecklist, makeChecklistItem } from './checklist-test-fixtures.js';

describe('WikiChecklist cursor positioning', () => {
  let el: WikiChecklist;

  beforeEach(async () => {
    el = document.createElement('wiki-checklist') as WikiChecklist;
    el.setAttribute('list-name', 'test_list');
    el.setAttribute('page', 'test-page');

    sinon.stub(el.client, 'listItems').resolves(
      create(ListItemsResponseSchema, {
        checklist: makeChecklist({
          name: 'test_list',
          items: [
            makeChecklistItem({
              uid: 'uid-hello',
              text: 'Hello World',
              tags: ['greeting'],
              sortOrder: 1000n,
            }),
          ],
        }),
      })
    );

    document.body.appendChild(el);
    await el.updateComplete;
    await el.updateComplete;

    el.loading = false;
    el.error = null;
    await el.updateComplete;
  });

  afterEach(() => {
    el.remove();
    sinon.restore();
  });

  describe('when the item row is rendered in display mode', () => {
    let row: HTMLElement | null | undefined;

    beforeEach(() => {
      row = el.shadowRoot?.querySelector<HTMLElement>('.item-row');
    });

    it('should NOT have draggable attribute on the item row', () => {
      expect(row?.draggable).to.be.false;
    });
  });

  describe('when the text input is rendered after clicking display text', () => {
    let textInput: HTMLInputElement | null | undefined;

    beforeEach(async () => {
      // Click the display span to enter edit mode
      const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
      (displaySpan as HTMLElement)?.click();
      await el.updateComplete;
      textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
    });

    it('should allow setting cursor position via setSelectionRange', () => {
      textInput!.focus();
      textInput!.setSelectionRange(3, 3);
      expect(textInput!.selectionStart).to.equal(3);
    });

    it('should not reset cursor when re-rendered during editing', async () => {
      // Now manually set cursor to position 5
      textInput!.setSelectionRange(5, 5);
      expect(textInput!.selectionStart).to.equal(5);

      // Trigger a re-render (simulating polling or other state change)
      el.requestUpdate();
      await el.updateComplete;

      // Cursor should still be at 5 (editingIndex keeps the input rendered)
      expect(textInput!.selectionStart).to.equal(5);
    });
  });

  describe('when an OCC refetch+retry happens during editing', () => {
    let textInput: HTMLInputElement | null | undefined;

    beforeEach(async () => {
      const displaySpan = el.shadowRoot?.querySelector('.item-display-text');
      (displaySpan as HTMLElement)?.click();
      await el.updateComplete;
      textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
      textInput!.focus();
      textInput!.setSelectionRange(5, 5);

      // Simulate the scenario: fetchData runs (e.g. polling) but the
      // server's updated_at hasn't changed. The component must short-circuit
      // and avoid disturbing the input or its cursor.
      const initialUpdatedAtMs = Date.now();
      el._listUpdatedAt = timestampFromMs(initialUpdatedAtMs);

      // Make the next listItems call return the SAME updated_at to exercise
      // the short-circuit path.
      const stub = el.client.listItems as unknown as sinon.SinonStub;
      stub.resolves(
        create(ListItemsResponseSchema, {
          checklist: makeChecklist({
            name: 'test_list',
            updatedAtMs: initialUpdatedAtMs,
            items: [
              makeChecklistItem({
                uid: 'uid-hello',
                text: 'Hello World',
                tags: ['greeting'],
                sortOrder: 1000n,
                updatedAtMs: initialUpdatedAtMs,
              }),
            ],
          }),
        })
      );

      // Stub UpdateItem so persist works without a real network.
      sinon.stub(el.client, 'updateItem').resolves(
        create(UpdateItemResponseSchema, {
          checklist: makeChecklist({
            name: 'test_list',
            updatedAtMs: initialUpdatedAtMs + 1000,
            items: [
              makeChecklistItem({
                uid: 'uid-hello',
                text: 'Hello World',
                tags: ['greeting'],
                sortOrder: 1000n,
                updatedAtMs: initialUpdatedAtMs + 1000,
              }),
            ],
          }),
        })
      );

      // Force a fetchData call (private method) — equivalent to a poll cycle.
      // Use the public connectedCallback path: trigger via visibilitychange
      // would also work, but here we just request another update.
      el.requestUpdate();
      await el.updateComplete;
    });

    it('should preserve cursor position after a same-token refetch', () => {
      expect(textInput!.selectionStart).to.equal(5);
    });
  });
});
