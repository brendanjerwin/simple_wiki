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

    it('should include mobile native dialog fullscreen rules', () => {
      expect(cssText).to.contain('@media (max-width: 768px)');
      expect(cssText).to.contain('width: 100%');
      expect(cssText).to.contain('height: 100dvh');
      expect(cssText).to.contain('max-height: 100dvh');
      expect(cssText).to.contain('inset: 0');
      expect(cssText).to.contain('dialog[open]');
      expect(cssText).to.contain('animation: none');
    });

    it('should keep mobile dialog content scrollable', () => {
      expect(cssText).to.contain('overflow-y: auto');
      expect(cssText).to.contain('min-height: 0');
    });
  });
});
