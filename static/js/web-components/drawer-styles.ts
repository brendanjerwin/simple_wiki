import { css } from 'lit';

/**
 * Shared CSS for drawer panels. Components apply this and add their own
 * width, background, and content styles on top.
 *
 * Desktop (>=768px): slides from the right edge.
 * Mobile (<768px): slides up from the bottom as a sheet.
 */
export const drawerCSS = css`
  .drawer-panel {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: var(--z-drawer);
    transform: translateX(100%);
    transition: transform 0.3s ease;
    display: flex;
    flex-direction: column;
  }

  .drawer-panel[open] {
    transform: translateX(0);
  }

  @media (max-width: 768px) {
    .drawer-panel {
      top: auto;
      left: 0;
      right: 0;
      bottom: 0;
      width: 100%;
      max-height: 60dvh;
      border-radius: 12px 12px 0 0;
      transform: translateY(100%);
    }

    .drawer-panel[open] {
      transform: translateY(0);
    }
  }
`;
