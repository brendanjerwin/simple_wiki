import { expect } from '@open-wc/testing';
import { dialogStyles, animationCSS } from './shared-styles.js';

describe('shared-styles', () => {
  describe('animationCSS', () => {
    let cssText: string;

    beforeEach(() => {
      cssText = animationCSS.cssText;
    });

    it('should contain fadeIn keyframe animation', () => {
      expect(cssText).to.contain('@keyframes fadeIn');
    });

    it('should contain slideIn keyframe animation', () => {
      expect(cssText).to.contain('@keyframes slideIn');
    });
  });

  describe('dialogStyles()', () => {
    let cssText: string;

    beforeEach(() => {
      const styles = dialogStyles();
      cssText = styles.map(s => s.cssText).join('');
    });

    it('should return a non-empty array', () => {
      expect(dialogStyles().length).to.be.greaterThan(0);
    });

    it('should include fadeIn keyframe animation', () => {
      expect(cssText).to.contain('@keyframes fadeIn');
    });

    it('should include slideIn keyframe animation', () => {
      expect(cssText).to.contain('@keyframes slideIn');
    });
  });
});
