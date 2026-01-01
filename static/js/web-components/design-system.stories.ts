/* eslint-disable @typescript-eslint/no-explicit-any */
import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html, css, LitElement } from 'lit';
import { colorCSS, typographyCSS, themeCSS } from './shared-styles.js';

// Demo component for showing the design system
class DesignSystemDemo extends LitElement {
  static override styles = [
    colorCSS,
    typographyCSS, 
    themeCSS,
    css`
      :host {
        display: block;
        padding: 20px;
      }

      .demo-grid {
        display: grid;
        gap: 20px;
        margin: 20px 0;
      }

      .demo-section {
        border: 1px solid #e0e0e0;
        border-radius: 8px;
        padding: 16px;
        background: #f8f9fa;
      }
      
      /* Dark containers need contrasting background for visibility */
      .demo-container {
        background: #1a1a1a;
        padding: 20px;
        border-radius: 8px;
        margin: 10px 0;
        position: relative;
        border: 2px solid #333;
      }
      
      /* Add visual indicators for examples */
      .demo-container::before {
        content: '👁️ Live Example';
        position: absolute;
        top: -10px;
        left: 10px;
        font-size: 10px;
        color: #888;
        background: #f8f9fa;
        padding: 2px 8px;
        border-radius: 4px;
        border: 1px solid #ddd;
      }

      .demo-title {
        font-size: 16px;
        font-weight: 600;
        margin-bottom: 12px;
        color: #333;
      }

      .color-swatch {
        display: inline-block;
        width: 40px;
        height: 40px;
        border-radius: 4px;
        margin-right: 8px;
        vertical-align: middle;
        border: 1px solid #ccc;
      }

      .color-info {
        display: inline-block;
        vertical-align: middle;
        font-family: monospace;
        font-size: 12px;
        margin-right: 16px;
      }

      .demo-container {
        margin: 8px 0;
      }

      .code-snippet {
        background: #f4f4f4;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 8px;
        font-family: monospace;
        font-size: 12px;
        margin: 8px 0;
        white-space: pre-wrap;
        overflow-x: auto;
        display: block;
        position: relative;
      }
      
      .code-snippet::before {
        content: '📋 Copy Code';
        position: absolute;
        top: -10px;
        right: 10px;
        font-size: 10px;
        color: #666;
        background: #f4f4f4;
        padding: 2px 8px;
        border-radius: 4px;
        border: 1px solid #ddd;
      }
      
      .code-snippet code {
        font-family: inherit;
        font-size: inherit;
        background: none;
        padding: 0;
      }

      .demo-row {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 8px 0;
      }
    `
  ];

  override render() {
    return html`<slot></slot>`;
  }
}

customElements.define('design-system-demo', DesignSystemDemo);

declare global {
  interface HTMLElementTagNameMap {
    'design-system-demo': DesignSystemDemo;
  }
}

const meta: Meta = {
  title: 'Design System/Style Guide',
  component: 'design-system-demo',
  parameters: {
    layout: 'fullscreen',
    docs: {
      description: {
        component: `
# Design System Style Guide

Our cohesive design language based on the system-info aesthetic. Use these patterns and utilities to build consistent interfaces.

## Philosophy
- **Dark Theme**: Translucent dark containers with subtle presence
- **Monospace Typography**: Technical content uses monospace fonts
- **Opacity Interactions**: Standalone containers fade in on hover
- **Compact Spacing**: Dense, information-rich layouts
- **Semantic Colors**: Clear color coding for states and meaning

## Quick Start
\`\`\`typescript
import { colorCSS, typographyCSS, themeCSS } from './shared-styles.js';

static override styles = [
  colorCSS,      // Color variables and design tokens
  typographyCSS, // Font families, sizes, and text colors
  themeCSS,      // Container patterns and spacing utilities
  css\`/* your component styles */\`
];
\`\`\`

## Container Types
- **ambient**: Subtle containers that fade in on hover (toasts, system info)
- **modal**: Always-visible containers for user attention (dialogs, alerts)
- **embedded**: Fully visible containers inside other components (error displays)
        `,
      },
    },
  },
};

export default meta;
type Story = StoryObj;

export const ColorPalette: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Color Palette</div>
        
        <h4>Background Colors</h4>
        <div class="demo-row">
          <span class="color-swatch" style="background: #2d2d2d;"></span>
          <span class="color-info">--color-background-primary: #2d2d2d</span>
          <pre class="code-snippet"><code>background: var(--color-background-primary);</code></pre>
        </div>
        <div class="demo-row">
          <span class="color-swatch" style="background: rgba(0, 0, 0, 0.5);"></span>
          <span class="color-info">--color-background-overlay: rgba(0, 0, 0, 0.5)</span>
          <pre class="code-snippet"><code>background: var(--color-background-overlay);</code></pre>
        </div>

        <h4>Border Colors</h4>
        <div class="demo-row">
          <span class="color-swatch" style="background: #404040;"></span>
          <span class="color-info">--color-border-primary: #404040</span>
          <pre class="code-snippet"><code>border: 1px solid var(--color-border-primary);</code></pre>
        </div>

        <h4>Text Colors</h4>
        <div class="demo-row">
          <span class="color-swatch" style="background: #e9ecef;"></span>
          <span class="color-info">--color-text-primary: #e9ecef</span>
          <pre class="code-snippet"><code>color: var(--color-text-primary);</code></pre>
        </div>
        <div class="demo-row">
          <span class="color-swatch" style="background: #adb5bd;"></span>
          <span class="color-info">--color-text-muted: #adb5bd</span>
          <pre class="code-snippet"><code>color: var(--color-text-muted);</code></pre>
        </div>

        <h4>Semantic Colors</h4>
        <div class="demo-row">
          <span class="color-swatch" style="background: #28a745;"></span>
          <span class="color-info">--color-success: #28a745</span>
          <pre class="code-snippet"><code>color: var(--color-success);</code></pre>
        </div>
        <div class="demo-row">
          <span class="color-swatch" style="background: #dc3545;"></span>
          <span class="color-info">--color-error: #dc3545</span>
          <pre class="code-snippet"><code>color: var(--color-error);</code></pre>
        </div>
        <div class="demo-row">
          <span class="color-swatch" style="background: #ffc107;"></span>
          <span class="color-info">--color-warning: #ffc107</span>
          <pre class="code-snippet"><code>color: var(--color-warning);</code></pre>
        </div>
        <div class="demo-row">
          <span class="color-swatch" style="background: #6c757d;"></span>
          <span class="color-info">--color-info: #6c757d</span>
          <pre class="code-snippet"><code>color: var(--color-info);</code></pre>
        </div>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Complete color palette with CSS custom properties. Copy the code snippets to use these colors in your components.',
      },
    },
  },
};

export const Typography: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Typography System</div>
        
        <h4>Font Families</h4>
        <div class="demo-container">
          <div class="font-system text-primary" style="font-size: 14px; margin: 8px 0;">
            System Font: The quick brown fox jumps over the lazy dog
          </div>
        </div>
        <pre class="code-snippet"><code>class="font-system"</code></pre>
        
        <div class="demo-container">
          <div class="font-mono text-primary" style="font-size: 14px; margin: 8px 0;">
            Monospace: The quick brown fox jumps over the lazy dog
          </div>
        </div>
        <pre class="code-snippet"><code>class="font-mono"</code></pre>

        <h4>Font Sizes</h4>
        <div class="demo-container">
          <div class="text-xs font-mono text-primary">Extra Small (10px): Technical details and fine print</div>
        </div>
        <pre class="code-snippet"><code>class="text-xs"</code></pre>
        
        <div class="demo-container">
          <div class="text-sm font-mono text-primary">Small (11px): Primary text for compact interfaces</div>
        </div>
        <pre class="code-snippet"><code>class="text-sm"</code></pre>
        
        <div class="demo-container">
          <div class="text-base font-mono text-primary">Base (12px): Headings and emphasized content</div>
        </div>
        <pre class="code-snippet"><code>class="text-base"</code></pre>

        <h4>Text Colors (with dark background)</h4>
        <div class="demo-container">
          <div style="display: flex; flex-direction: column; gap: 8px;">
            <div class="text-primary font-mono text-sm">Primary text for main content</div>
            <div class="text-muted font-mono text-xs">Muted text for secondary content</div>
            <div class="text-success font-mono text-xs">Success: Operation completed</div>
            <div class="text-error font-mono text-xs">Error: Something went wrong</div>
            <div class="text-warning font-mono text-xs">Warning: Attention required</div>
            <div class="text-info font-mono text-xs">Info: Additional information</div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="text-primary"
class="text-muted"
class="text-success"
class="text-error"
class="text-warning"
class="text-info"</code></pre>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Typography system with font families, sizes, and semantic colors. Use font-mono for technical content.',
      },
    },
  },
};

export const Containers: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Container Patterns</div>
        
        <h4>Ambient Container (fades in on hover)</h4>
        <div class="demo-container">
          <div class="container container-ambient panel">
            <div class="text-primary font-mono text-sm">Ambient container</div>
            <div class="text-muted font-mono text-xs">Used for toasts, system-info - hover to see opacity change</div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-ambient panel"</code></pre>

        <h4>Modal Container (always visible)</h4>
        <div class="demo-container">
          <div class="container container-modal panel">
            <div class="text-primary font-mono text-sm">Modal container</div>
            <div class="text-muted font-mono text-xs">Used for dialogs, popovers - always at 90% opacity</div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-modal panel"</code></pre>

        <h4>Embedded Container (always visible)</h4>
        <div class="demo-container">
          <div class="container container-embedded panel">
            <div class="text-primary font-mono text-sm">Embedded container</div>
            <div class="text-muted font-mono text-xs">Always visible when inside other components</div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-embedded panel"</code></pre>

        <h4>Compact Panel (Ambient)</h4>
        <div class="demo-container">
          <div class="container container-ambient panel-compact">
            <div class="text-primary font-mono text-xs">Compact ambient panel with less padding</div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-ambient panel-compact"</code></pre>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Container patterns for different use cases. Use container-ambient for toasts/system-info (hover to reveal), container-modal for dialogs/popovers (always visible), and container-embedded for error displays inside other components.',
      },
    },
  },
};

export const ComponentCompositions: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Component Compositions</div>
        
        <h4>Toast-style Notification (Ambient)</h4>
        <div class="demo-container">
          <div class="container container-ambient panel gap-sm" style="border-left: 3px solid var(--color-success); max-width: 320px;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span style="color: var(--color-success);">✅</span>
              <div class="text-primary font-mono text-sm">Build completed successfully</div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-ambient panel gap-sm"
style="border-left: 3px solid var(--color-success);"</code></pre>

        <h4>Error Display</h4>
        <div class="demo-container">
          <div class="container container-embedded panel gap-sm" style="border: 1px solid var(--color-error);">
            <div style="display: flex; align-items: flex-start; gap: 12px;">
              <span class="text-error" style="font-size: 20px;">⚠️</span>
              <div style="flex: 1;">
                <div class="text-primary font-mono text-sm">Connection failed</div>
                <div class="text-muted font-mono text-xs">Unable to reach server</div>
              </div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-embedded panel gap-sm"
style="border: 1px solid var(--color-error);"</code></pre>

        <h4>System Info Panel (Ambient)</h4>
        <div class="demo-container">
          <div class="container container-ambient panel-compact gap-xs">
            <div style="display: flex; align-items: center; gap: 6px;">
              <div style="width: 6px; height: 6px; background: var(--color-success); border-radius: 50%;"></div>
              <span class="text-muted font-mono text-xs">Status:</span>
              <span class="text-primary font-mono text-xs">Online</span>
            </div>
            <div style="display: flex; align-items: center; gap: 6px;">
              <span class="text-muted font-mono text-xs">Rate:</span>
              <span class="text-primary font-mono text-xs">42.3/s</span>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-ambient panel-compact gap-xs"</code></pre>

        <h4>Dialog Content (Modal)</h4>
        <div class="demo-container">
          <div class="container container-modal panel gap-sm" style="max-width: 400px;">
            <div style="text-align: center;">
              <div style="font-size: 24px; margin-bottom: 12px; color: var(--color-warning);">⚠️</div>
              <div class="text-primary font-mono text-base" style="font-weight: 600; margin-bottom: 8px;">Confirm Action</div>
              <div class="text-muted font-mono text-sm">This action cannot be undone.</div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>class="container container-modal panel gap-sm"</code></pre>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Real-world compositions showing how to combine containers, typography, and colors for common UI patterns.',
      },
    },
  },
};

export const InteractiveExamples: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Interactive Examples</div>
        
        <h4>Hover Effects Demonstration</h4>
        <p style="color: #666; font-size: 12px; margin-bottom: 16px;">Hover over the containers below to see the opacity effects in action:</p>
        
        <div class="demo-container">
          <div style="display: flex; gap: 20px; flex-wrap: wrap;">
            <div class="container container-ambient panel">
              <div class="text-primary font-mono text-sm">Ambient Container</div>
              <div class="text-muted font-mono text-xs">Hover me!</div>
            </div>
            
            <div class="container container-modal panel">
              <div class="text-primary font-mono text-sm">Modal Container</div>
              <div class="text-muted font-mono text-xs">Always visible</div>
            </div>
            
            <div class="container container-embedded panel">
              <div class="text-primary font-mono text-sm">Embedded Container</div>
              <div class="text-muted font-mono text-xs">No opacity effects</div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>.container-ambient   // Fades in on hover
.container-modal     // Always at 90% opacity
.container-embedded  // Always at 100% opacity</code></pre>
        
        <h4>Real Component Examples</h4>
        <p style="color: #666; font-size: 12px; margin-bottom: 16px;">These examples show how the design system looks in actual components:</p>
        
        <div class="demo-container">
          <div class="container container-ambient panel gap-sm" style="border-left: 3px solid var(--color-success); max-width: 300px;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span style="color: var(--color-success); font-size: 16px;">✅</span>
              <div class="text-primary font-mono text-sm">Build completed successfully</div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>/* Toast-style notification */
&lt;div class="container container-ambient panel gap-sm"&gt;
  &lt;div class="text-primary font-mono text-sm"&gt;Message&lt;/div&gt;
&lt;/div&gt;</code></pre>
        
        <div class="demo-container">
          <div class="container container-embedded panel gap-sm" style="border: 1px solid var(--color-error);">
            <div style="display: flex; align-items: flex-start; gap: 12px;">
              <span class="text-error" style="font-size: 20px;">⚠️</span>
              <div style="flex: 1;">
                <div class="text-primary font-mono text-sm">Connection failed</div>
                <div class="text-muted font-mono text-xs">Check your network connection</div>
              </div>
            </div>
          </div>
        </div>
        <pre class="code-snippet"><code>/* Error display */
&lt;div class="container container-embedded panel gap-sm"&gt;
  &lt;span class="text-error"&gt;⚠️&lt;/span&gt;
  &lt;div class="text-primary font-mono text-sm"&gt;Error&lt;/div&gt;
&lt;/div&gt;</code></pre>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Interactive examples showing hover effects and real component usage. Try hovering over the containers to see the ambient opacity behavior.',
      },
    },
  },
};

export const UsageExamples: Story = {
  render: () => html`
    <design-system-demo>
      <div class="demo-section">
        <div class="demo-title">Usage Examples</div>
        
        <h4>Complete Component Template</h4>
        <pre class="code-snippet"><code>import { colorCSS, typographyCSS, themeCSS } from './shared-styles.js';

export class MyComponent extends LitElement {
  static override styles = [
    colorCSS,
    typographyCSS,
    themeCSS,
    css\`
      :host {
        display: block;
      }
      
      /* Custom component styles go here */
    \`
  ];

  override render() {
    return html\`
      <div class="container container-ambient panel gap-sm">
        <div class="text-primary font-mono text-sm">Main content</div>
        <div class="text-muted font-mono text-xs">Secondary content</div>
      </div>
    \`;
  }
}</code></pre>

        <h4>Quick Reference</h4>
        <pre class="code-snippet"><code>/* Containers */
.container                 // Base dark container
.container-ambient         // Fades in on hover (toasts, system-info)
.container-modal           // Always visible (dialogs, popovers)
.container-embedded        // No opacity effects (error displays)
.panel                     // Standard padding
.panel-compact             // Less padding

/* Typography */
.font-mono                 // Monospace font
.text-xs .text-sm .text-base  // Sizes 10px/11px/12px
.text-primary .text-muted      // Text colors
.text-success .text-error      // Semantic colors

/* Spacing */
.gap-xs .gap-sm .gap-base     // Gap utilities</code></pre>
      </div>
    </design-system-demo>
  `,
  parameters: {
    docs: {
      description: {
        story: 'Copy-paste examples and quick reference for implementing the design system in your components.',
      },
    },
  },
};