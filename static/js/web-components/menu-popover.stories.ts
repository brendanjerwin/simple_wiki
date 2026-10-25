import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import './menu-popover.js';

const meta: Meta = {
  title: 'Components/MenuPopover',
  component: 'menu-popover',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
A shared viewport-aware dropdown/popover primitive. All menus in the wiki
(agent selector, editor toolbar, hashtag bubbles) render through it, so
placement behavior exists exactly once.

**Placement behavior:**
- Opens below/beside its anchor by default (\`bottom-start\`).
- **Flips above** when the space below the anchor is tighter than a 200px
  minimum and above has more room.
- **Mirrors to the far edge** (\`*-end\`) when the near side cannot fit the
  surface, so near-right-edge anchors grow the menu leftward.
- Caps the surface height to available viewport space; long lists scroll
  inside the menu (\`max-height\` + \`overflow-y: auto\`).
- Re-places on window resize and scroll while open.
- Outside click and Escape dispatch a cancellable
  \`menu-popover-close-request\`; the host owns \`open\` and aria-expanded.

**Positioning model:** \`position: absolute\` inside the host with offsets
computed from anchor and host viewport rects, so placement stays exact under
transformed ancestors (where \`position: fixed\` would silently re-anchor).
Tradeoff: an ancestor with \`overflow: hidden\` can clip the menu.

**Interactive testing:** click a trigger below, then resize the browser
window while its menu is open — the menu re-places inside the viewport. The
"bottom right + tall list" trigger exercises flip, mirror, and scroll cap
together.
        `,
      },
    },
  },
};
export default meta;
type Story = StoryObj;

interface DemoRow {
  id: string;
  label: string;
  /** CSS positioning applied to the wrapper, e.g. 'top: 78vh;'. */
  anchorStyle: string;
  /** Option count so one demo hits the max-height scroll cap. */
  optionCount: number;
}

const demos: DemoRow[] = [
  { id: 'plain', label: 'Room below', anchorStyle: '', optionCount: 3 },
  { id: 'bottom', label: 'Near bottom (flips above)', anchorStyle: 'top: 78vh;', optionCount: 3 },
  { id: 'right', label: 'Near right edge (mirrors left)', anchorStyle: 'left: 90%;', optionCount: 3 },
  {
    id: 'corner',
    label: 'Bottom-right + tall list (flip + mirror + scroll cap)',
    anchorStyle: 'top: 70vh; left: 88%;',
    optionCount: 40,
  },
  {
    id: 'top',
    label: 'Near top (top-start with more room below stays below)',
    anchorStyle: 'top: 4px;',
    optionCount: 3,
  },
];

/** Interactive wrapper: owns a trigger + popover pair, mirrors real hosts. */
class MenuDemo extends LitElement {
  @property() declare label: string;
  @property({ type: Number }) declare optionCount: number;
  /** Extra CSS applied to the host so the trigger lands near an edge. */
  @property() declare anchorStyle: string;

  @state() private open = false;

  constructor() {
    super();
    this.label = '';
    this.optionCount = 0;
    this.anchorStyle = '';
    this.open = false;
  }

  static override readonly styles = css`
    :host {
      display: inline-block;
      position: relative;
    }
    .demo-trigger {
      padding: 6px 12px;
      cursor: pointer;
      white-space: nowrap;
    }
    .menu-opt {
      display: block;
      width: 100%;
      padding: 8px 14px;
      border: none;
      background: none;
      text-align: left;
      cursor: pointer;
      font: inherit;
      color: inherit;
    }
    .menu-opt:hover {
      background: rgba(255, 255, 255, 0.08);
    }
    menu-popover {
      min-width: 220px;
    }
  `;
  @state() private trigger: HTMLButtonElement | null = null;

  protected override firstUpdated(): void {
    // Anchor style applies to the host inline style (absolute position).
    this.style.cssText += `; ${this.anchorStyle}`;
    this.trigger = this.shadowRoot?.querySelector<HTMLButtonElement>('.demo-trigger') ?? null;
  }

  private readonly _onTriggerClick = (): void => {
    this.open = !this.open;
  };

  override render(): unknown {
    return html`
      <button class="demo-trigger" @click=${this._onTriggerClick}>${this.label}</button>
      <menu-popover
        .anchor=${this.trigger}
        .open=${this.open}
        surfaceRole="menu"
        @menu-popover-close-request=${() => (this.open = false)}
      >
        ${Array.from(
      { length: this.optionCount },
      (_, i) => html`<button class="menu-opt">${`Option ${i + 1}`}</button>`,
    )}
      </menu-popover>
    `;
  }
}
customElements.define('menu-demo', MenuDemo);

declare global {
  interface HTMLElementTagNameMap {
    'menu-demo': MenuDemo;
  }
}

const pageStyles = html`
  <style>
    #menu-popover-demos {
      padding: 24px 40px 60vh 24px;
      min-height: 180vh; /* forces page scroll so capture-scroll re-place is testable */
      position: relative;
    }
    .demo-row {
      margin: 24px 0;
    }
  </style>
`;

export const InteractiveDemos: Story = {
  render: () => html`${pageStyles}
    <div id="menu-popover-demos">
      <p>
        Each trigger owns its own <code>&lt;menu-popover&gt;</code>. Click to
        toggle; close with outside click, Escape, or the toggle again.
      </p>
      ${demos.map(
    (demo) => html`
          <div class="demo-row">
            <menu-demo
              .label=${demo.label}
              .optionCount=${demo.optionCount}
              .anchorStyle=${demo.anchorStyle}
            ></menu-demo>
          </div>
        `,
  )}
    </div>`,
  parameters: {
    docs: {
      description: {
        story: 'Live toggles covering every placement path. Resize the window while open; scroll while open.',
      },
    },
  },
};