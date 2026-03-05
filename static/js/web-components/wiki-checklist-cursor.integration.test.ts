/**
 * Integration test: verify that clicking in a checklist text input
 * actually positions the cursor at the click point.
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
import { GetFrontmatterResponseSchema } from '../gen/api/v1/frontmatter_pb.js';

describe('WikiChecklist cursor positioning', () => {
  let el: WikiChecklist;

  beforeEach(async () => {
    el = document.createElement('wiki-checklist') as WikiChecklist;
    el.setAttribute('list-name', 'test_list');
    el.setAttribute('page', 'test-page');

    sinon
      .stub(el.client, 'getFrontmatter')
      .resolves(create(GetFrontmatterResponseSchema, { frontmatter: {} }));

    document.body.appendChild(el);
    await el.updateComplete;

    el.loading = false;
    el.error = null;
    el.items = [
      { text: 'Hello World', checked: false, tags: ['greeting'] },
    ];
    await el.updateComplete;
  });

  afterEach(() => {
    el.remove();
    sinon.restore();
  });

  describe('when the text input is rendered', () => {
    let row: HTMLElement | null | undefined;
    let textInput: HTMLInputElement | null | undefined;

    beforeEach(() => {
      row = el.shadowRoot?.querySelector<HTMLElement>('.item-row');
      textInput = el.shadowRoot?.querySelector<HTMLInputElement>('.item-text');
    });

    it('should NOT have draggable attribute on the item row', () => {
      expect(row?.draggable).to.be.false;
    });

    it('should allow setting cursor position via setSelectionRange', () => {
      textInput!.focus();
      textInput!.setSelectionRange(3, 3);
      expect(textInput!.selectionStart).to.equal(3);
    });

    it('should not reset cursor when re-rendered during editing', async () => {
      // Focus and enter edit mode
      textInput!.focus();
      textInput!.dispatchEvent(new FocusEvent('focus'));
      await el.updateComplete;

      // Now manually set cursor to position 5
      textInput!.setSelectionRange(5, 5);
      expect(textInput!.selectionStart).to.equal(5);

      // Trigger a re-render (simulating polling or other state change)
      el.requestUpdate();
      await el.updateComplete;

      // Cursor should still be at 5 (noChange prevents value clobber)
      expect(textInput!.selectionStart).to.equal(5);
    });
  });
});
