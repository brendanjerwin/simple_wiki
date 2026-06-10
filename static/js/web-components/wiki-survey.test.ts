import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-survey.js';
import type { WikiSurvey } from './wiki-survey.js';
import { create } from '@bufbuild/protobuf';
import {
  GetSurveyResponseSchema,
  SubmitSurveyResponseResponseSchema,
  SurveyFieldSchema,
  SurveyResponseSchema,
  SurveySchema,
} from '../gen/api/v1/survey_pb.js';
import { TimestampSchema, type Timestamp } from '@bufbuild/protobuf/wkt';
import type { JsonObject, JsonValue } from '@bufbuild/protobuf';
import { asRecord } from './survey-data-service.js';

describe('WikiSurvey', () => {
  let el: WikiSurvey;

  function buildElement(page = 'test-page', surveyName = 'my_survey'): WikiSurvey {
    const freshEl = document.createElement('wiki-survey') as WikiSurvey;
    freshEl.setAttribute('name', surveyName);
    freshEl.setAttribute('page', page);
    return freshEl;
  }

  function timestampFromIso(isoValue: string) {
    const date = new Date(isoValue);
    if (Number.isNaN(date.getTime())) return undefined;
    return create(TimestampSchema, {
      seconds: BigInt(Math.floor(date.getTime() / 1000)),
      nanos: (date.getTime() % 1000) * 1_000_000,
    });
  }

  function isPrimitiveJsonValue(value: unknown): value is JsonValue {
    if (typeof value === 'number') {
      return Number.isFinite(value);
    }
    return value === null || typeof value === 'string' || typeof value === 'boolean';
  }

  function jsonObjectFromRecord(record: Record<string, unknown> | null): JsonObject {
    const out: JsonObject = {};
    if (!record) return out;
    for (const [key, value] of Object.entries(record)) {
      if (isPrimitiveJsonValue(value)) {
        out[key] = value;
      }
    }
    return out;
  }

  function surveyFromFrontmatter(frontmatter: JsonObject, surveyName = 'my_survey') {
    const surveyObj = asRecord(asRecord(frontmatter['surveys'])?.[surveyName]) ?? {};
    const rawFields = Array.isArray(surveyObj['fields']) ? surveyObj['fields'] : [];
    const rawResponses = Array.isArray(surveyObj['responses']) ? surveyObj['responses'] : [];

    return create(SurveySchema, {
      name: surveyName,
      question: typeof surveyObj['question'] === 'string' ? surveyObj['question'] : '',
      closed: surveyObj['closed'] === true,
      fields: rawFields
        .map(asRecord)
        .filter((field): field is Record<string, unknown> => field !== null)
        .map(field => {
          const init: {
            name: string;
            type: string;
            options: string[];
            label?: string;
            required?: boolean;
            min?: number;
            max?: number;
          } = {
            name: typeof field['name'] === 'string' ? field['name'] : '',
            type: typeof field['type'] === 'string' ? field['type'] : 'text',
            options: Array.isArray(field['options']) ? field['options'].filter((option): option is string => typeof option === 'string') : [],
          };
          if (typeof field['label'] === 'string') init.label = field['label'];
          if (field['required'] === true) init.required = true;
          if (typeof field['min'] === 'number') init.min = field['min'];
          if (typeof field['max'] === 'number') init.max = field['max'];
          return create(SurveyFieldSchema, init);
        }),
      responses: rawResponses
        .map(asRecord)
        .filter((response): response is Record<string, unknown> => response !== null)
        .map(response => {
          const init: {
            user: string;
            anonymous: boolean;
            values: JsonObject;
            submittedAt?: Timestamp;
          } = {
            user: typeof response['user'] === 'string' ? response['user'] : '',
            anonymous: response['anonymous'] === true,
            values: jsonObjectFromRecord(asRecord(response['values'])),
          };
          const submittedAt: Timestamp | undefined = typeof response['submitted_at'] === 'string' ? timestampFromIso(response['submitted_at']) : undefined;
          if (submittedAt) init.submittedAt = submittedAt;
          return create(SurveyResponseSchema, init);
        }),
    });
  }

  function stubGetSurvey(target: WikiSurvey, frontmatter: JsonObject = {}, surveyName = 'my_survey'): SinonStub {
    return sinon
      .stub(target.client, 'getSurvey')
      .resolves(create(GetSurveyResponseSchema, { survey: surveyFromFrontmatter(frontmatter, surveyName) }));
  }

  function stubSubmitResponse(target: WikiSurvey, frontmatter: JsonObject = {}, surveyName = 'my_survey'): SinonStub {
    return sinon
      .stub(target.client, 'submitResponse')
      .resolves(create(SubmitSurveyResponseResponseSchema, { survey: surveyFromFrontmatter(frontmatter, surveyName) }));
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
    stubGetSurvey(el);
    document.body.appendChild(el);
    await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    expect(el).to.exist;
  });

  describe('when connected with a page attribute', () => {
    let getSurveyStub: SinonStub;

    beforeEach(async () => {
      el = buildElement();
      getSurveyStub = stubGetSurvey(el);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should call getSurvey', () => {
      expect(getSurveyStub).to.have.been.calledOnce;
    });

    it('should call getSurvey with the correct page', () => {
      const args = getSurveyStub.getCall(0).args[0] as { page: string };
      expect(args.page).to.equal('test-page');
    });

    it('should call getSurvey with the correct survey name', () => {
      const args = getSurveyStub.getCall(0).args[0] as { name: string };
      expect(args.name).to.equal('my_survey');
    });
  });

  describe('when survey has no config in frontmatter', () => {
    beforeEach(async () => {
      el = buildElement();
      stubGetSurvey(el, {});
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
      stubGetSurvey(el, surveyFrontmatter);
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

  describe('when survey has a select field', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'What do you prefer?',
          fields: [
            { name: 'protein_preference', type: 'select', label: 'Protein preference', options: ['Chicken', 'Beef', 'Fish'] },
          ],
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      stubGetSurvey(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render a <select> element', () => {
      const select = el.shadowRoot?.querySelector('select');
      expect(select).to.exist;
    });

    it('should render the options inside the select', () => {
      const options = el.shadowRoot?.querySelectorAll('select option');
      const optionValues = Array.from(options ?? []).map(o => (o as HTMLOptionElement).value);
      expect(optionValues).to.include('Chicken');
      expect(optionValues).to.include('Beef');
      expect(optionValues).to.include('Fish');
    });

    it('should render the human-readable label text', () => {
      const label = el.shadowRoot?.querySelector('label[for="field-protein_preference"]');
      expect(label?.textContent?.trim()).to.include('Protein preference');
    });
  });

  describe('when a field has a label', () => {
    const surveyFrontmatter: JsonObject = {
      surveys: {
        my_survey: {
          question: 'How are you?',
          fields: [
            { name: 'mood_score', type: 'number', label: 'How do you feel? (1–5)' },
          ],
        },
      },
    } as unknown as JsonObject;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      stubGetSurvey(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
    });

    it('should render the label text instead of the field name', () => {
      const label = el.shadowRoot?.querySelector('label[for="field-mood_score"]');
      expect(label?.textContent?.trim()).to.include('How do you feel? (1–5)');
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
      stubGetSurvey(el, surveyFrontmatter);
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
      stubGetSurvey(el, surveyFrontmatter);
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
      stubGetSurvey(el, surveyFrontmatter);
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

  describe('when getSurvey fails', () => {
    beforeEach(async () => {
      el = buildElement();
      sinon.stub(el.client, 'getSurvey').rejects(new Error('network error'));
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

  describe('accessibility', () => {
    describe('when survey has fields and user is logged in', () => {
      const surveyFrontmatter: JsonObject = {
        surveys: {
          my_survey: {
            question: 'How was it?',
            fields: [
              { name: 'rating', type: 'number', min: 1, max: 5 },
              { name: 'protein_preference', type: 'text' },
              { name: 'favorite_food', type: 'text', label: "What's your favorite food?" },
              { name: 'agreed', type: 'boolean' },
              { name: 'mood', type: 'choice', options: ['happy', 'sad'] },
            ],
          },
        },
      } as unknown as JsonObject;

      beforeEach(async () => {
        globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
        el = buildElement();
        stubGetSurvey(el, surveyFrontmatter);
        document.body.appendChild(el);
        await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
      });

      describe('field labels', () => {
        it('should have a label associated with the number input', () => {
          const label = el.shadowRoot?.querySelector('label[for="field-rating"]');
          expect(label).to.exist;
        });

        it('should have a label associated with the text input', () => {
          const label = el.shadowRoot?.querySelector('label[for="field-protein_preference"]');
          expect(label).to.exist;
        });

        it('should have a label associated with the checkbox input', () => {
          const label = el.shadowRoot?.querySelector('label[for="field-agreed"]');
          expect(label).to.exist;
        });

        it('should have a label associated with the select input', () => {
          const label = el.shadowRoot?.querySelector('label[for="field-mood"]');
          expect(label).to.exist;
        });

        describe('when field has no explicit label', () => {
          let label: HTMLLabelElement | null | undefined;

          beforeEach(() => {
            label = el.shadowRoot?.querySelector('label[for="field-protein_preference"]');
          });

          it('should auto-humanize field names', () => {
            expect(label?.textContent?.trim()).to.equal('Protein Preference');
          });
        });

        describe('when field has an explicit label', () => {
          let label: HTMLLabelElement | null | undefined;

          beforeEach(() => {
            label = el.shadowRoot?.querySelector('label[for="field-favorite_food"]');
          });

          it('should render the explicit label verbatim', () => {
            expect(label?.textContent?.trim()).to.equal("What's your favorite food?");
          });
        });
      });

      describe('boolean fields', () => {
        it('should use input type="checkbox" for boolean fields', () => {
          const checkbox = el.shadowRoot?.querySelector('input[type="checkbox"]');
          expect(checkbox).to.exist;
        });

        it('should have an id matching the label for attribute on the checkbox', () => {
          const checkbox = el.shadowRoot?.querySelector('#field-agreed');
          expect(checkbox?.getAttribute('type')).to.equal('checkbox');
        });
      });

      describe('field grouping', () => {
        it('should have role="group" on survey fields container', () => {
          const group = el.shadowRoot?.querySelector('.survey-fields');
          expect(group?.getAttribute('role')).to.equal('group');
        });

        it('should have aria-labelledby referencing the survey question', () => {
          const group = el.shadowRoot?.querySelector('.survey-fields');
          expect(group?.getAttribute('aria-labelledby')).to.equal('survey-question-my_survey');
        });

        it('should have the survey question element with the matching id', () => {
          const question = el.shadowRoot?.querySelector('#survey-question-my_survey');
          expect(question).to.exist;
        });
      });

      describe('submit status region', () => {
        it('should have role="status" on the submit status element', () => {
          const status = el.shadowRoot?.querySelector('.submit-status');
          expect(status?.getAttribute('role')).to.equal('status');
        });

        it('should have aria-live="polite" on the submit status element', () => {
          const status = el.shadowRoot?.querySelector('.submit-status');
          expect(status?.getAttribute('aria-live')).to.equal('polite');
        });
      });
    });

    describe('when getSurvey fails', () => {
      beforeEach(async () => {
        el = buildElement();
        sinon.stub(el.client, 'getSurvey').rejects(new Error('network error'));
        document.body.appendChild(el);
        await waitUntil(
          () => el.error !== null,
          'error should be set',
          { timeout: 3000 }
        );
        await el.updateComplete;
      });

      it('should have role="alert" on the error wrapper', () => {
        const wrapper = el.shadowRoot?.querySelector('.error-wrapper');
        expect(wrapper?.getAttribute('role')).to.equal('alert');
      });
    });

    describe('when survey has required fields', () => {
      const requiredFrontmatter: JsonObject = {
        surveys: {
          my_survey: {
            question: 'Required fields survey',
            fields: [
              { name: 'rating', type: 'number', required: true },
              { name: 'notes', type: 'text', required: true },
              { name: 'agreed', type: 'boolean', required: true },
              { name: 'mood', type: 'choice', options: ['happy', 'sad'], required: true },
            ],
          },
        },
      } as unknown as JsonObject;

      beforeEach(async () => {
        globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
        el = buildElement();
        stubGetSurvey(el, requiredFrontmatter);
        document.body.appendChild(el);
        await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
      });

      it('should have aria-required="true" on the required number input', () => {
        const input = el.shadowRoot?.querySelector('#field-rating');
        expect(input?.getAttribute('aria-required')).to.equal('true');
      });

      it('should have aria-required="true" on the required text input', () => {
        const input = el.shadowRoot?.querySelector('#field-notes');
        expect(input?.getAttribute('aria-required')).to.equal('true');
      });

      it('should have aria-required="true" on the required checkbox', () => {
        const input = el.shadowRoot?.querySelector('#field-agreed');
        expect(input?.getAttribute('aria-required')).to.equal('true');
      });

      it('should have aria-required="true" on the required select', () => {
        const input = el.shadowRoot?.querySelector('#field-mood');
        expect(input?.getAttribute('aria-required')).to.equal('true');
      });

      it('should show a visual required indicator in the number field label', () => {
        const label = el.shadowRoot?.querySelector('label[for="field-rating"]');
        const indicator = label?.querySelector('.required-indicator');
        expect(indicator).to.exist;
      });

      it('should show a visual required indicator in the text field label', () => {
        const label = el.shadowRoot?.querySelector('label[for="field-notes"]');
        const indicator = label?.querySelector('.required-indicator');
        expect(indicator).to.exist;
      });

      it('should show a visual required indicator in the boolean field label', () => {
        const label = el.shadowRoot?.querySelector('label[for="field-agreed"]');
        const indicator = label?.querySelector('.required-indicator');
        expect(indicator).to.exist;
      });

      it('should show a visual required indicator in the choice field label', () => {
        const label = el.shadowRoot?.querySelector('label[for="field-mood"]');
        const indicator = label?.querySelector('.required-indicator');
        expect(indicator).to.exist;
      });

      it('should hide the required indicator from screen readers via aria-hidden', () => {
        const indicator = el.shadowRoot?.querySelector('.required-indicator');
        expect(indicator?.getAttribute('aria-hidden')).to.equal('true');
      });
    });

    describe('when survey has non-required fields', () => {
      const optionalFrontmatter: JsonObject = {
        surveys: {
          my_survey: {
            question: 'Optional fields survey',
            fields: [
              { name: 'rating', type: 'number' },
            ],
          },
        },
      } as unknown as JsonObject;

      beforeEach(async () => {
        globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
        el = buildElement();
        stubGetSurvey(el, optionalFrontmatter);
        document.body.appendChild(el);
        await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });
      });

      it('should not have aria-required on non-required inputs', () => {
        const input = el.shadowRoot?.querySelector('#field-rating');
        expect(input?.getAttribute('aria-required')).to.be.null;
      });

      it('should not show a required indicator in the label', () => {
        const label = el.shadowRoot?.querySelector('label[for="field-rating"]');
        const indicator = label?.querySelector('.required-indicator');
        expect(indicator).to.not.exist;
      });
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
    let getSurveyStub: SinonStub;
    let submitResponseStub: SinonStub;

    beforeEach(async () => {
      globalThis.simple_wiki = { ...(globalThis.simple_wiki ?? {}), username: 'alice' };
      el = buildElement();
      getSurveyStub = stubGetSurvey(el, surveyFrontmatter);
      submitResponseStub = stubSubmitResponse(el, surveyFrontmatter);
      document.body.appendChild(el);
      await waitUntil(() => !el.loading, 'fetch should complete', { timeout: 3000 });

      const btn = el.shadowRoot?.querySelector('.submit-btn') as HTMLButtonElement | null;
      btn?.click();

      await waitUntil(
        () => submitResponseStub.callCount > 0,
        'submitResponse should be called',
        { timeout: 3000 }
      );
      await el.updateComplete;
    });

    it('should not fetch the survey again', () => {
      expect(getSurveyStub).to.have.been.calledOnce;
    });

    it('should call submitResponse', () => {
      expect(submitResponseStub).to.have.been.calledOnce;
    });

    it('should submit to the correct page', () => {
      const args = submitResponseStub.getCall(0).args[0] as { page: string };
      expect(args.page).to.equal('test-page');
    });

    it('should submit to the correct survey name', () => {
      const args = submitResponseStub.getCall(0).args[0] as { surveyName: string };
      expect(args.surveyName).to.equal('my_survey');
    });

    it('should submit the field values', () => {
      const args = submitResponseStub.getCall(0).args[0] as { values?: JsonObject };
      expect(args.values).to.deep.equal({});
    });

    it('should submit a non-anonymous response', () => {
      const args = submitResponseStub.getCall(0).args[0] as { anonymous: boolean };
      expect(args.anonymous).to.equal(false);
    });
  });
});
