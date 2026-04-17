import { expect } from '@open-wc/testing';
import {
  extractSurveyData,
  findUserResponse,
  upsertResponse,
  asRecord,
  type SurveyData,
  type SurveyResponse,
} from './survey-data-service.js';
import type { JsonObject } from '@bufbuild/protobuf';

describe('asRecord', () => {
  describe('when given a plain object', () => {
    it('should return the object', () => {
      expect(asRecord({ a: 1 })).to.deep.equal({ a: 1 });
    });
  });

  describe('when given null', () => {
    it('should return null', () => {
      expect(asRecord(null)).to.be.null;
    });
  });

  describe('when given an array', () => {
    it('should return null', () => {
      expect(asRecord([1, 2, 3])).to.be.null;
    });
  });

  describe('when given a string', () => {
    it('should return null', () => {
      expect(asRecord('hello')).to.be.null;
    });
  });
});

describe('extractSurveyData', () => {
  describe('when frontmatter has no surveys key', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData({} as unknown as JsonObject, 'my_survey');
    });

    it('should return empty question', () => {
      expect(result.question).to.equal('');
    });

    it('should return empty fields', () => {
      expect(result.fields).to.deep.equal([]);
    });

    it('should return empty responses', () => {
      expect(result.responses).to.deep.equal([]);
    });

    it('should return closed as false', () => {
      expect(result.closed).to.be.false;
    });
  });

  describe('when the named survey does not exist', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData({ surveys: { other_survey: {} } } as unknown as JsonObject, 'my_survey');
    });

    it('should return empty question', () => {
      expect(result.question).to.equal('');
    });
  });

  describe('when the survey has a question and fields', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData(
        {
          surveys: {
            pastry_2026_w15: {
              question: 'Rate this week\'s pastries',
              fields: [
                { name: 'rating', type: 'number', min: 1, max: 5 },
                { name: 'notes', type: 'text' },
              ],
            },
          },
        } as unknown as JsonObject,
        'pastry_2026_w15'
      );
    });

    it('should extract the question', () => {
      expect(result.question).to.equal('Rate this week\'s pastries');
    });

    it('should extract the fields', () => {
      expect(result.fields).to.have.length(2);
    });

    it('should extract the first field name', () => {
      expect(result.fields[0]?.name).to.equal('rating');
    });

    it('should extract the first field type', () => {
      expect(result.fields[0]?.type).to.equal('number');
    });

    it('should extract the first field min', () => {
      expect(result.fields[0]?.min).to.equal(1);
    });

    it('should extract the first field max', () => {
      expect(result.fields[0]?.max).to.equal(5);
    });

    it('should extract the second field name', () => {
      expect(result.fields[1]?.name).to.equal('notes');
    });

    it('should extract the second field type', () => {
      expect(result.fields[1]?.type).to.equal('text');
    });
  });

  describe('when the survey has responses', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData(
        {
          surveys: {
            my_survey: {
              question: 'Test question',
              fields: [],
              responses: [
                {
                  user: 'brendan',
                  anonymous: false,
                  submitted_at: '2026-04-18T14:22:00Z',
                  values: { rating: 4, notes: 'great' },
                },
              ],
            },
          },
        } as unknown as JsonObject,
        'my_survey'
      );
    });

    it('should extract one response', () => {
      expect(result.responses).to.have.length(1);
    });

    it('should extract the response user', () => {
      expect(result.responses[0]?.user).to.equal('brendan');
    });

    it('should extract the response submitted_at', () => {
      expect(result.responses[0]?.submitted_at).to.equal('2026-04-18T14:22:00Z');
    });

    it('should extract the response values', () => {
      expect(result.responses[0]?.values).to.deep.equal({ rating: 4, notes: 'great' });
    });
  });

  describe('when closed is true', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData(
        {
          surveys: {
            my_survey: {
              question: 'Test',
              fields: [],
              closed: true,
            },
          },
        } as unknown as JsonObject,
        'my_survey'
      );
    });

    it('should return closed as true', () => {
      expect(result.closed).to.be.true;
    });
  });

  describe('when fields contain a choice type with options', () => {
    let result: SurveyData;

    beforeEach(() => {
      result = extractSurveyData(
        {
          surveys: {
            my_survey: {
              question: 'Test',
              fields: [
                { name: 'color', type: 'choice', options: ['red', 'blue', 'green'] },
              ],
            },
          },
        } as unknown as JsonObject,
        'my_survey'
      );
    });

    it('should extract options', () => {
      expect(result.fields[0]?.options).to.deep.equal(['red', 'blue', 'green']);
    });
  });
});

describe('findUserResponse', () => {
  const responses: SurveyResponse[] = [
    {
      user: 'alice',
      anonymous: false,
      submitted_at: '2026-04-18T10:00:00Z',
      values: { rating: 5 },
    },
    {
      user: 'bob',
      anonymous: false,
      submitted_at: '2026-04-18T11:00:00Z',
      values: { rating: 3 },
    },
  ];

  describe('when the user has an existing response', () => {
    it('should return the response', () => {
      const result = findUserResponse(responses, 'alice');
      expect(result?.user).to.equal('alice');
    });
  });

  describe('when the user does not have a response', () => {
    it('should return null', () => {
      expect(findUserResponse(responses, 'charlie')).to.be.null;
    });
  });

  describe('when username is empty', () => {
    it('should return null', () => {
      expect(findUserResponse(responses, '')).to.be.null;
    });
  });
});

describe('upsertResponse', () => {
  describe('when no existing response for the user', () => {
    let result: SurveyResponse[];

    beforeEach(() => {
      result = upsertResponse([], 'alice', { rating: 4 });
    });

    it('should append one response', () => {
      expect(result).to.have.length(1);
    });

    it('should set the user field', () => {
      expect(result[0]?.user).to.equal('alice');
    });

    it('should set the values', () => {
      expect(result[0]?.values).to.deep.equal({ rating: 4 });
    });

    it('should set anonymous to false', () => {
      expect(result[0]?.anonymous).to.be.false;
    });

    it('should set submitted_at to a non-empty string', () => {
      expect(result[0]?.submitted_at).to.be.a('string').and.not.empty;
    });
  });

  describe('when user already has a response', () => {
    const existing: SurveyResponse[] = [
      {
        user: 'alice',
        anonymous: false,
        submitted_at: '2026-04-18T10:00:00Z',
        values: { rating: 3 },
      },
    ];
    let result: SurveyResponse[];

    beforeEach(() => {
      result = upsertResponse(existing, 'alice', { rating: 5, notes: 'updated' });
    });

    it('should not increase the response count', () => {
      expect(result).to.have.length(1);
    });

    it('should replace the values', () => {
      expect(result[0]?.values).to.deep.equal({ rating: 5, notes: 'updated' });
    });

    it('should update the submitted_at', () => {
      expect(result[0]?.submitted_at).to.not.equal('2026-04-18T10:00:00Z');
    });
  });

  describe('when multiple users have responses', () => {
    const existing: SurveyResponse[] = [
      { user: 'alice', anonymous: false, submitted_at: '2026-04-18T10:00:00Z', values: { rating: 3 } },
      { user: 'bob', anonymous: false, submitted_at: '2026-04-18T11:00:00Z', values: { rating: 4 } },
    ];
    let result: SurveyResponse[];

    beforeEach(() => {
      result = upsertResponse(existing, 'alice', { rating: 5 });
    });

    it('should preserve the other user response', () => {
      const bob = result.find(r => r.user === 'bob');
      expect(bob?.values).to.deep.equal({ rating: 4 });
    });

    it('should update only alice response', () => {
      const alice = result.find(r => r.user === 'alice');
      expect(alice?.values).to.deep.equal({ rating: 5 });
    });
  });
});
