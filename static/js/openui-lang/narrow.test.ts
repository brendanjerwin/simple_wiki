/**
 * Tests for the runtime narrowing helpers used by component renderers.
 */
import { expect } from '@open-wc/testing';

import { asPlan, toPlanStep } from './narrow.js';

describe('toPlanStep', () => {
 describe('when the input is malformed', () => {
  it('should reject null', () => {
   expect(toPlanStep(null)).to.be.undefined;
  });

  it('should reject primitives', () => {
   expect(toPlanStep(42)).to.be.undefined;
   expect(toPlanStep('click')).to.be.undefined;
  });

  it('should reject arrays', () => {
   expect(toPlanStep([{ type: 'click' }])).to.be.undefined;
  });

  it('should reject objects without a type', () => {
   expect(toPlanStep({})).to.be.undefined;
  });

  it('should reject non-string types', () => {
   expect(toPlanStep({ type: 7 })).to.be.undefined;
  });
 });

 describe('when string fields are present and valid', () => {
  it('should copy each string field onto the step', () => {
   const step = toPlanStep({
    type: 'custom_action',
    target: 'panel-a',
    message: 'hello',
    url: 'https://example.com',
    statementId: 'st-1',
    name: 'myAction',
   });
   expect(step).to.deep.equal({
    type: 'custom_action',
    target: 'panel-a',
    message: 'hello',
    url: 'https://example.com',
    statementId: 'st-1',
    name: 'myAction',
   });
  });
 });

 describe('when string fields have the wrong type', () => {
  it('should omit them from the step', () => {
   const step = toPlanStep({
    type: 'click',
    target: 7,
    message: null,
    url: { deep: true },
    statementId: [],
    name: false,
   });
   expect(step).to.deep.equal({ type: 'click' });
  });
 });

 describe('when targets is present', () => {
  it('should filter entries down to strings', () => {
   const step = toPlanStep({ type: 'click', targets: ['a', 5, 'b', null] });
   expect(step?.targets).to.deep.equal(['a', 'b']);
  });
 });

 describe('when refType is present', () => {
  it('should accept query and mutation values', () => {
   expect(toPlanStep({ type: 'click', refType: 'query' })?.refType).to.equal('query');
   expect(toPlanStep({ type: 'click', refType: 'mutation' })?.refType).to.equal('mutation');
  });

  it('should reject other values', () => {
   expect(toPlanStep({ type: 'click', refType: 'delete' })?.refType).to.be.undefined;
  });
 });

 describe('when params is a plain object', () => {
  it('should copy params so step mutation does not leak to raw', () => {
   const raw = { type: 'custom_action', name: 'myAction', params: { $count: 1 } };
   const step = toPlanStep(raw) as { params?: Record<string, unknown> };
   step.params!['$count'] = 99;
   expect((raw.params as Record<string, unknown>)['$count']).to.equal(1);
  });
 });

 describe('when params is not a plain object', () => {
  it('should omit null', () => {
   expect(toPlanStep({ type: 'click', params: null })?.params).to.be.undefined;
  });

  it('should omit arrays', () => {
   expect(toPlanStep({ type: 'click', params: [1, 2] })?.params).to.be.undefined;
  });

  it('should omit strings', () => {
   expect(toPlanStep({ type: 'click', params: 'nope' })?.params).to.be.undefined;
  });
 });

 describe('when valueAST is present', () => {
  it('should carry the key with a null value', () => {
   const step = toPlanStep({ type: 'click', valueAST: null });
   expect(step).to.deep.equal({ type: 'click', valueAST: null });
  });
 });


});

describe('asPlan', () => {
 describe('when the input is not a steps array', () => {
  it('should return undefined', () => {
   expect(asPlan(null)).to.be.undefined;
   expect(asPlan({})).to.be.undefined;
   expect(asPlan({ steps: 'nope' })).to.be.undefined;
  });
 });

 describe('when steps contain malformed entries', () => {
  it('should filter out the malformed and keep the valid', () => {
   const plan = asPlan({ steps: [{ type: 'click' }, 'junk', null, { type: 'noop' }] });
   expect(plan).to.deep.equal({ steps: [{ type: 'click' }, { type: 'noop' }] });
  });
 });
});