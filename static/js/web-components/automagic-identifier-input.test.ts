import { html, fixture, expect } from '@open-wc/testing';
import { stub, restore, type SinonStub, useFakeTimers, type SinonFakeTimers } from 'sinon';
import './automagic-identifier-input.js';
import type { AutomagicIdentifierInput } from './automagic-identifier-input.js';
import type { ExistingPageInfo } from '../gen/api/v1/page_management_pb.js';

describe('AutomagicIdentifierInput', () => {
  let element: AutomagicIdentifierInput;
  let generateIdentifierStub: SinonStub;

  beforeEach(() => {
    generateIdentifierStub = stub();
  });

  afterEach(() => {
    restore();
  });

  describe('when created with default properties', () => {
    beforeEach(async () => {
      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
        ></automagic-identifier-input>
      `);
    });

    it('should have empty title', () => {
      expect(element.title).to.equal('');
    });

    it('should have empty identifier', () => {
      expect(element.identifier).to.equal('');
    });

    it('should be in automagic mode by default', () => {
      expect(element.automagicMode).to.be.true;
    });

    it('should indicate identifier is unique', () => {
      expect(element.isUnique).to.be.true;
    });

    it('should render title input', () => {
      const titleInput = element.shadowRoot?.querySelector('input[name="title"]');
      expect(titleInput).to.exist;
    });

    it('should render identifier input', () => {
      const identifierInput = element.shadowRoot?.querySelector('input[name="identifier"]');
      expect(identifierInput).to.exist;
    });

    it('should render automagic toggle button with sparkle icon', () => {
      const button = element.shadowRoot?.querySelector('.automagic-button');
      expect(button).to.exist;
      expect(button?.querySelector('.fa-wand-magic-sparkles')).to.exist;
    });
  });

  describe('when title input changes', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = useFakeTimers();
      generateIdentifierStub.resolves({
        identifier: 'my_page',
        isUnique: true,
      });

      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
        ></automagic-identifier-input>
      `);

      const titleInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
      if (titleInput) {
        titleInput.value = 'My Page';
        titleInput.dispatchEvent(new Event('input', { bubbles: true }));
      }
    });

    afterEach(() => {
      clock.restore();
    });

    it('should update title property', () => {
      expect(element.title).to.equal('My Page');
    });

    describe('after debounce period', () => {
      beforeEach(async () => {
        await clock.tickAsync(300);
        await element.updateComplete;
      });

      it('should call generateIdentifier with title', () => {
        expect(generateIdentifierStub).to.have.been.calledWith('My Page');
      });

      it('should update identifier from response', () => {
        expect(element.identifier).to.equal('my_page');
      });
    });
  });

  describe('when toggling automagic mode', () => {
    beforeEach(async () => {
      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
        ></automagic-identifier-input>
      `);

      const button = element.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
      button?.click();
      await element.updateComplete;
    });

    it('should switch to manual mode', () => {
      expect(element.automagicMode).to.be.false;
    });

    it('should show pen icon in button', () => {
      const button = element.shadowRoot?.querySelector('.automagic-button');
      expect(button?.querySelector('.fa-pen')).to.exist;
    });

    it('should allow identifier input to be editable', () => {
      const identifierInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="identifier"]');
      expect(identifierInput?.readOnly).to.be.false;
    });
  });

  describe('when identifier already exists', () => {
    const existingPage: ExistingPageInfo = {
      identifier: 'my_page',
      title: 'My Existing Page',
      container: 'container1',
      $typeName: 'api.v1.ExistingPageInfo'
    };

    beforeEach(async () => {
      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
          .isUnique=${false}
          .existingPage=${existingPage}
          identifier="my_page"
        ></automagic-identifier-input>
      `);
    });

    it('should display conflict warning', () => {
      const warning = element.shadowRoot?.querySelector('.conflict-warning');
      expect(warning).to.exist;
    });

    it('should show link to existing page', () => {
      const link = element.shadowRoot?.querySelector('.conflict-warning a');
      expect(link?.getAttribute('href')).to.equal('/my_page');
    });
  });

  describe('when in manual mode and identifier is edited', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = useFakeTimers();
      generateIdentifierStub.resolves({
        identifier: 'custom_id',
        isUnique: true,
      });

      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
          .automagicMode=${false}
        ></automagic-identifier-input>
      `);

      const identifierInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="identifier"]');
      if (identifierInput) {
        identifierInput.value = 'custom_id';
        identifierInput.dispatchEvent(new Event('input', { bubbles: true }));
      }
    });

    afterEach(() => {
      clock.restore();
    });

    it('should update identifier property', () => {
      expect(element.identifier).to.equal('custom_id');
    });

    describe('after debounce period', () => {
      beforeEach(async () => {
        await clock.tickAsync(300);
        await element.updateComplete;
      });

      it('should check identifier availability', () => {
        expect(generateIdentifierStub).to.have.been.calledWith('custom_id');
      });
    });
  });

  describe('when disabled', () => {
    beforeEach(async () => {
      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
          .disabled=${true}
        ></automagic-identifier-input>
      `);
    });

    it('should disable title input', () => {
      const titleInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
      expect(titleInput?.disabled).to.be.true;
    });

    it('should disable identifier input', () => {
      const identifierInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="identifier"]');
      expect(identifierInput?.disabled).to.be.true;
    });

    it('should disable automagic button', () => {
      const button = element.shadowRoot?.querySelector<HTMLButtonElement>('.automagic-button');
      expect(button?.disabled).to.be.true;
    });
  });

  describe('events', () => {
    describe('when title changes', () => {
      let eventSpy: SinonStub;

      beforeEach(async () => {
        eventSpy = stub();
        element = await fixture(html`
          <automagic-identifier-input
            .generateIdentifier=${generateIdentifierStub}
            @title-change=${eventSpy}
          ></automagic-identifier-input>
        `);

        const titleInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'New Title';
          titleInput.dispatchEvent(new Event('input', { bubbles: true }));
        }
      });

      it('should dispatch title-change event', () => {
        expect(eventSpy).to.have.been.called;
      });

      it('should include new title in event detail', () => {
        const event = eventSpy.firstCall.args[0];
        expect(event.detail.title).to.equal('New Title');
      });
    });

    describe('when identifier changes via automagic', () => {
      let eventSpy: SinonStub;
      let clock: SinonFakeTimers;

      beforeEach(async () => {
        clock = useFakeTimers();
        eventSpy = stub();
        generateIdentifierStub.resolves({
          identifier: 'generated_id',
          isUnique: true,
        });

        element = await fixture(html`
          <automagic-identifier-input
            .generateIdentifier=${generateIdentifierStub}
            @identifier-change=${eventSpy}
          ></automagic-identifier-input>
        `);

        const titleInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
        if (titleInput) {
          titleInput.value = 'My Title';
          titleInput.dispatchEvent(new Event('input', { bubbles: true }));
        }

        await clock.tickAsync(300);
        await element.updateComplete;
      });

      afterEach(() => {
        clock.restore();
      });

      it('should dispatch identifier-change event', () => {
        expect(eventSpy).to.have.been.called;
      });

      it('should include new identifier in event detail', () => {
        const event = eventSpy.firstCall.args[0];
        expect(event.detail.identifier).to.equal('generated_id');
      });
    });
  });

  describe('when generateIdentifier fails', () => {
    let clock: SinonFakeTimers;

    beforeEach(async () => {
      clock = useFakeTimers();
      generateIdentifierStub.resolves({
        error: new Error('Network failed'),
      });

      element = await fixture(html`
        <automagic-identifier-input
          .generateIdentifier=${generateIdentifierStub}
        ></automagic-identifier-input>
      `);

      const titleInput = element.shadowRoot?.querySelector<HTMLInputElement>('input[name="title"]');
      if (titleInput) {
        titleInput.value = 'My Page';
        titleInput.dispatchEvent(new Event('input', { bubbles: true }));
      }

      await clock.tickAsync(300);
      await element.updateComplete;
    });

    afterEach(() => {
      clock.restore();
    });

    it('should display an error message', () => {
      const errorDisplay = element.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });

    it('should set the automagicError property', () => {
      expect(element.automagicError).to.exist;
    });

    it('should preserve the error message', () => {
      expect(element.automagicError?.message).to.equal('Network failed');
    });

    it('should set the failed goal description', () => {
      expect(element.automagicError?.failedGoalDescription).to.equal('generating identifier');
    });
  });
});
