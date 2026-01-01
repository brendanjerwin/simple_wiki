import { expect } from '@open-wc/testing';
import { dialogStyles, animationCSS } from './shared-styles.js';

describe('shared-styles', () => {
  describe('animationCSS', () => {
    it('should contain fadeIn keyframe animation', () => {
      const cssText = animationCSS.cssText;
      expect(cssText).to.contain('@keyframes fadeIn');
      expect(cssText).to.contain('from');
      expect(cssText).to.contain('opacity: 0');
      expect(cssText).to.contain('opacity: 1');
    });

    it('should contain slideIn keyframe animation', () => {
      const cssText = animationCSS.cssText;
      expect(cssText).to.contain('@keyframes slideIn');
      expect(cssText).to.contain('transform: translateY(-20px)');
      expect(cssText).to.contain('transform: translateY(0)');
    });
  });

  describe('dialogStyles()', () => {
    describe('when building dialog styles', () => {
      it('should include fadeIn keyframe animation', () => {
        const styles = dialogStyles();
        const cssText = styles.map(s => s.cssText).join('');
        expect(cssText).to.contain('@keyframes fadeIn');
      });

      it('should include slideIn keyframe animation', () => {
        const styles = dialogStyles();
        const cssText = styles.map(s => s.cssText).join('');
        expect(cssText).to.contain('@keyframes slideIn');
      });

      it('should include both animations from animationCSS', () => {
        const styles = dialogStyles();
        const cssText = styles.map(s => s.cssText).join('');
        expect(cssText).to.contain('@keyframes fadeIn');
        expect(cssText).to.contain('@keyframes slideIn');
      });

      it('should include component styles when provided', () => {
        const styles = dialogStyles();
        expect(styles.length).to.be.greaterThan(0);
      });
    });
  });
});
