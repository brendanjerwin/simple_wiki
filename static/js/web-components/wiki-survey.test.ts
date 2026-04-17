import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-survey.js';
import type { WikiSurvey } from './wiki-survey.js';
import { create } from '@bufbuild/protobuf';
import {
  GetFrontmatterResponseSchema,
  MergeFrontmatterResponseSchema,
} from '../gen/api/v1/frontmatter_pb.js';
import type { JsonObject } from '@bufbuild/protobuf';

describe('WikiSurvey', () => {
  let el: WikiSurvey;

  function buildElement(page = 'test-page', surveyName = 'my_survey'): WikiSurvey {
    const freshEl = document.createElement('wiki-survey') as WikiSurvey;
    freshEl.setAttribute('name', surveyName);
    freshEl.setAttribute('page', page);
    return freshEl;
  }

  function stubGetFrontmatter(target: WikiSurvey, frontmatter: JsonObject = {}): SinonStub {
    return sinon
      .stub(target.client, 'getFrontmatter')
      .resolves(create(GetFrontmatterResponseSchema, { frontmatter }));
  }

  function stubMergeFrontmatter(target: WikiSurvey, frontmatter: JsonObject = {}): SinonStub {
    return sinon
      .stub(target.client, 'mergeFrontmatter')
      .resolves(create(MergeFrontmatterResponseSchema, { frontmatter }));
  }

  afterEach(() => {
    sinon.restore();
    if (el) {
      el.remove();
    }
    if (globalThis.simple_wiki) {
      globalThis.simple_wiki.username = '';
    }
  });

  it('should exist', async () => {
    el = buildElement();
    stubGetFrontmatter(el);
    document.body.appendChild(el);
    await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    expect(el).to.exist;
  });

  describe('when connected with a page attribute', () => {
    let getFrontmatterStub: SinonStub;

    beforeEach(async () => {
      el = buildElement();
      getFrontmatterStub = stubGetFrontmatter(el);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should call getFrontmatter', () => {
      expect(getFrontmatterStub).to.have.been.calledOnce;
    });

    it('should call getFrontmatter with the correct page', () => {
      const args = getFrontmatterStub.getCall(0).args[0] as { page: string };
      expect(args.page).to.equal('test-page');
    });
  });

  describe('when survey has no config in frontmatter', () => {
    beforeEach(async () => {
      el = buildElement();
      stubGetFrontmatter(el, {});
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the not-configured message', () => {
      const msg = el.shadowRoot?.querySelector('.not-configured');
      expect(msg).to.exist;
    });
  });

  describe('when survey is configured with a question and user is logged in', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'How was it?',
          fields: [
            { name: 'rating', type: 'number', min: 1, max: 5 },
            { name: 'notes', type: 'text' },
          ],
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      stubGetFrontmatter(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the survey question', () => {
      const question = el.shadowRoot?.querySelector('.survey-question');
      expect(question?.textContent?.trim()).to.equal('How was it?');
    });

    it('should render the submit button', () => {
      const btn = el.shadowRoot?.querySelector('.submit-btn');
      expect(btn).to.exist;
    });

    it('should render a field input for rating', () => {
      const input = el.shadowRoot?.querySelector('input[type="number"]');
      expect(input).to.exist;
    });

    it('should render a field input for notes', () => {
      const input = el.shadowRoot?.querySelector('input[type="text"]');
      expect(input).to.exist;
    });
  });

  describe('when user is not logged in', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'How was it?',
          fields: [{ name: 'rating', type: 'number' }],
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: '' };
      el = buildElement();
      stubGetFrontmatter(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the login-required message', () => {
      const msg = el.shadowRoot?.querySelector('.login-required');
      expect(msg).to.exist;
    });

    it('should not render the submit button', () => {
      const btn = el.shadowRoot?.querySelector('.submit-btn');
      expect(btn).to.not.exist;
    });
  });

  describe('when survey is closed', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'Closed survey',
          fields: [{ name: 'rating', type: 'number' }],
          closed: true,
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      stubGetFrontmatter(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the closed notice', () => {
      const notice = el.shadowRoot?.querySelector('.closed-notice');
      expect(notice).to.exist;
    });

    it('should not render the submit button', () => {
      const btn = el.shadowRoot?.querySelector('.submit-btn');
      expect(btn).to.not.exist;
    });
  });

  describe('when existing responses are present', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'Rate it',
          fields: [{ name: 'rating', type: 'number' }],
          responses: [
            {
              user: 'alice',
              anonymous: false,
              submitted_at: '2026-04-18T10:00:00Z',
              values: { rating: 4 },
            },
          ],
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      stubGetFrontmatter(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the responses section', () => {
      const section = el.shadowRoot?.querySelector('.responses-section');
      expect(section).to.exist;
    });

    it('should render the response user', () => {
      const user = el.shadowRoot?.querySelector('.response-user');
      expect(user?.textContent?.trim()).to.equal('alice');
    });
  });

  describe('when getFrontmatter fails', () => {
    beforeEach(async () => {
      el = buildElement();
      sinon.stub(el.client, 'getFrontmatter').rejects(new Error('network error'));
      document.body.appendChild(el);
      await waitUntil(
        () => el.error !== null,
        'error should be set',
        { timeout: 3000 }
      );
      await el.updateComplete;
    });

    it('should render the error display', () => {
      const errorDisplay = el.shadowRoot?.querySelector('error-display');
      expect(errorDisplay).to.exist;
    });
  });

  describe('when submitting a response', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'Rate it',
          fields: [{ name: 'rating', type: 'number', min: 1, max: 5 }],
        },
      },
    } as unknown as JsonObject;
    let getFrontmatterStub: SinonStub;
    let mergeFrontmatterStub: SinonStub;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      getFrontmatterStub = stubGetFrontmatter(el, surveyFrontmatter);
      mergeFrontmatterStub = stubMergeFrontmatter(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });

      const btn = el.shadowRoot?.querySelector('.submit-btn') as HTMLButtonElement | null;
      btn?.click();

      await waitUntil(
        () => mergeFrontmatterStub.callCount > 0,
        'mergeFrontmatter should be called',
        { timeout: 3000 }
      );
      await el.updateComplete;
    });

    it('should call getFrontmatter a second time for read-modify-write', () => {
      expect(getFrontmatterStub.callCount).to.be.greaterThan(1);
    });

    it('should call mergeFrontmatter', () => {
      expect(mergeFrontmatterStub).to.have.been.calledOnce;
    });

    it('should include the username in the merged payload', () => {
      const args = mergeFrontmatterStub.getCall(0).args[0] as { frontmatter: JsonObject };
      const surveys = args.frontmatter['surveys'] as JsonObject;
      const survey = surveys['my_survey'] as JsonObject;
      const responses = survey['responses'] as JsonObject[];
      expect(responses[0]?.['user']).to.equal('alice');
    });
  });
});
