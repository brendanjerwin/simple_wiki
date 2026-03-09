import { expect } from '@open-wc/testing';
import {
  detectColumnType,
  parseNumericValue,
  parseDateValue,
  parseCurrencyValue,
  parsePercentageValue,
} from './column-type-detector.js';
import type { ColumnTypeInfo } from './column-type-detector.js';

describe('column-type-detector', () => {

  describe('detectColumnType', () => {
    let result: ColumnTypeInfo;

    describe('when given numeric values', () => {

      describe('when all values are integers', () => {
        beforeEach(() => {
          result = detectColumnType(['1', '2', '3', '100']);
        });

        it('should detect number type', () => {
          expect(result.detectedType).to.equal('number');
        });

        it('should have high confidence', () => {
          expect(result.confidenceRatio).to.equal(1);
        });
      });

      describe('when values include decimals', () => {
        beforeEach(() => {
          result = detectColumnType(['1.5', '2.0', '3.14']);
        });

        it('should detect number type', () => {
          expect(result.detectedType).to.equal('number');
        });
      });

      describe('when values include negative numbers', () => {
        beforeEach(() => {
          result = detectColumnType(['-1', '2', '-3.5']);
        });

        it('should detect number type', () => {
          expect(result.detectedType).to.equal('number');
        });
      });

      describe('when values include comma-separated numbers', () => {
        beforeEach(() => {
          result = detectColumnType(['1,000', '2,500', '10,000']);
        });

        it('should detect number type', () => {
          expect(result.detectedType).to.equal('number');
        });
      });
    });

    describe('when given currency values', () => {

      describe('when all values have dollar signs', () => {
        beforeEach(() => {
          result = detectColumnType(['$9.99', '$24.50', '$100.00']);
        });

        it('should detect currency type', () => {
          expect(result.detectedType).to.equal('currency');
        });

        it('should have high confidence', () => {
          expect(result.confidenceRatio).to.equal(1);
        });
      });

      describe('when values have euro signs', () => {
        beforeEach(() => {
          result = detectColumnType(['€10.00', '€25.50']);
        });

        it('should detect currency type', () => {
          expect(result.detectedType).to.equal('currency');
        });
      });

      describe('when values have pound signs', () => {
        beforeEach(() => {
          result = detectColumnType(['£10.00', '£25.50']);
        });

        it('should detect currency type', () => {
          expect(result.detectedType).to.equal('currency');
        });
      });

      describe('when values mix positive and negative with symbol-then-negative format', () => {
        beforeEach(() => {
          result = detectColumnType(['$100.00', '€-10.50', '$25.00', '-£5.00']);
        });

        it('should detect currency type', () => {
          expect(result.detectedType).to.equal('currency');
        });
      });
    });

    describe('when given percentage values', () => {

      describe('when all values have percent signs', () => {
        beforeEach(() => {
          result = detectColumnType(['50%', '75%', '100%']);
        });

        it('should detect percentage type', () => {
          expect(result.detectedType).to.equal('percentage');
        });

        it('should have high confidence', () => {
          expect(result.confidenceRatio).to.equal(1);
        });
      });

      describe('when values include decimals', () => {
        beforeEach(() => {
          result = detectColumnType(['12.5%', '33.3%']);
        });

        it('should detect percentage type', () => {
          expect(result.detectedType).to.equal('percentage');
        });
      });
    });

    describe('when given date values', () => {

      describe('when values are ISO date format', () => {
        beforeEach(() => {
          result = detectColumnType(['2024-01-15', '2024-02-20', '2024-03-10']);
        });

        it('should detect date type', () => {
          expect(result.detectedType).to.equal('date');
        });

        it('should have high confidence', () => {
          expect(result.confidenceRatio).to.equal(1);
        });
      });

      describe('when values are US date format', () => {
        beforeEach(() => {
          result = detectColumnType(['01/15/2024', '02/20/2024', '03/10/2024']);
        });

        it('should detect date type', () => {
          expect(result.detectedType).to.equal('date');
        });
      });

      describe('when values are human-readable dates', () => {
        beforeEach(() => {
          result = detectColumnType(['Jan 15, 2024', 'Feb 20, 2024', 'Mar 10, 2024']);
        });

        it('should detect date type', () => {
          expect(result.detectedType).to.equal('date');
        });
      });
    });

    describe('when given text values', () => {

      describe('when values are plain text', () => {
        beforeEach(() => {
          result = detectColumnType(['Alice', 'Bob', 'Charlie']);
        });

        it('should detect text type', () => {
          expect(result.detectedType).to.equal('text');
        });
      });

      describe('when values are mixed types below threshold', () => {
        beforeEach(() => {
          result = detectColumnType(['hello', '42', '$10', 'world', 'test']);
        });

        it('should detect text type', () => {
          expect(result.detectedType).to.equal('text');
        });
      });
    });

    describe('when given empty values mixed with typed values', () => {

      describe('when some values are empty among numbers', () => {
        beforeEach(() => {
          result = detectColumnType(['1', '', '3', '', '5']);
        });

        it('should detect number type (ignoring empties)', () => {
          expect(result.detectedType).to.equal('number');
        });

        it('should calculate confidence from non-empty values only', () => {
          expect(result.confidenceRatio).to.equal(1);
        });
      });
    });

    describe('when given all empty values', () => {
      beforeEach(() => {
        result = detectColumnType(['', '', '']);
      });

      it('should detect text type', () => {
        expect(result.detectedType).to.equal('text');
      });

      it('should have zero confidence', () => {
        expect(result.confidenceRatio).to.equal(0);
      });
    });

    describe('when given an empty array', () => {
      beforeEach(() => {
        result = detectColumnType([]);
      });

      it('should detect text type', () => {
        expect(result.detectedType).to.equal('text');
      });

      it('should have zero confidence', () => {
        expect(result.confidenceRatio).to.equal(0);
      });
    });
  });

  describe('parseNumericValue', () => {
    let result: number;

    describe('when given an integer string', () => {
      beforeEach(() => {
        result = parseNumericValue('42');
      });

      it('should return the number', () => {
        expect(result).to.equal(42);
      });
    });

    describe('when given a decimal string', () => {
      beforeEach(() => {
        result = parseNumericValue('3.14');
      });

      it('should return the number', () => {
        expect(result).to.be.closeTo(3.14, 0.001);
      });
    });

    describe('when given a negative number', () => {
      beforeEach(() => {
        result = parseNumericValue('-7.5');
      });

      it('should return the number', () => {
        expect(result).to.equal(-7.5);
      });
    });

    describe('when given a comma-separated number', () => {
      beforeEach(() => {
        result = parseNumericValue('1,234,567');
      });

      it('should return the number without commas', () => {
        expect(result).to.equal(1234567);
      });
    });

    describe('when given non-numeric text', () => {
      beforeEach(() => {
        result = parseNumericValue('hello');
      });

      it('should return NaN', () => {
        expect(result).to.be.NaN;
      });
    });
  });

  describe('parseCurrencyValue', () => {
    let result: number;

    describe('when given a dollar amount', () => {
      beforeEach(() => {
        result = parseCurrencyValue('$9.99');
      });

      it('should return the numeric value', () => {
        expect(result).to.be.closeTo(9.99, 0.001);
      });
    });

    describe('when given a euro amount', () => {
      beforeEach(() => {
        result = parseCurrencyValue('€25.50');
      });

      it('should return the numeric value', () => {
        expect(result).to.be.closeTo(25.50, 0.001);
      });
    });

    describe('when given a negative currency value with leading minus', () => {
      beforeEach(() => {
        result = parseCurrencyValue('-$15.00');
      });

      it('should return the negative numeric value', () => {
        expect(result).to.equal(-15);
      });
    });

    describe('when given a negative currency value with symbol-then-minus', () => {
      beforeEach(() => {
        result = parseCurrencyValue('€-10.50');
      });

      it('should return the negative numeric value', () => {
        expect(result).to.be.closeTo(-10.50, 0.001);
      });
    });

    describe('when given a value with commas', () => {
      beforeEach(() => {
        result = parseCurrencyValue('$1,234.56');
      });

      it('should return the numeric value', () => {
        expect(result).to.be.closeTo(1234.56, 0.001);
      });
    });

    describe('when given non-currency text', () => {
      beforeEach(() => {
        result = parseCurrencyValue('hello');
      });

      it('should return NaN', () => {
        expect(result).to.be.NaN;
      });
    });
  });

  describe('parsePercentageValue', () => {
    let result: number;

    describe('when given a whole percentage', () => {
      beforeEach(() => {
        result = parsePercentageValue('75%');
      });

      it('should return the numeric value', () => {
        expect(result).to.equal(75);
      });
    });

    describe('when given a decimal percentage', () => {
      beforeEach(() => {
        result = parsePercentageValue('33.3%');
      });

      it('should return the numeric value', () => {
        expect(result).to.be.closeTo(33.3, 0.001);
      });
    });

    describe('when given text without percent sign', () => {
      beforeEach(() => {
        result = parsePercentageValue('hello');
      });

      it('should return NaN', () => {
        expect(result).to.be.NaN;
      });
    });
  });

  describe('parseDateValue', () => {
    let result: number;

    describe('when given an ISO date', () => {
      beforeEach(() => {
        result = parseDateValue('2024-01-15');
      });

      it('should return epoch milliseconds', () => {
        expect(result).to.be.a('number');
        expect(result).to.not.be.NaN;
      });

      it('should parse the correct date', () => {
        const date = new Date(result);
        expect(date.getUTCFullYear()).to.equal(2024);
        expect(date.getUTCMonth()).to.equal(0);
        expect(date.getUTCDate()).to.equal(15);
      });
    });

    describe('when given a US format date', () => {
      beforeEach(() => {
        result = parseDateValue('01/15/2024');
      });

      it('should return epoch milliseconds', () => {
        expect(result).to.be.a('number');
        expect(result).to.not.be.NaN;
      });
    });

    describe('when given a human-readable date', () => {
      beforeEach(() => {
        result = parseDateValue('Jan 15, 2024');
      });

      it('should return epoch milliseconds', () => {
        expect(result).to.be.a('number');
        expect(result).to.not.be.NaN;
      });
    });

    describe('when given non-date text', () => {
      beforeEach(() => {
        result = parseDateValue('hello');
      });

      it('should return NaN', () => {
        expect(result).to.be.NaN;
      });
    });
  });
});
