import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
// Side-effect import: registers wiki-page-app-mic-button (customElements.define
// lives in library.ts; the type-only import below is erased at runtime).
import '../openui-lang/library.js';
import type { WikiPageAppMicButton } from '../openui-lang/library.js';

const meta: Meta = {
  title: 'Components/WikiPageAppMicButton',
  component: 'wiki-page-app-mic-button',
  parameters: {
    layout: 'centered',
    docs: {
      description: {
        component: [
          'The interactive voice-input button behind the wiki library\'s `MicButton(label, template?, mode?, disabled?, onResult?, lang?, interim?)` OpenUI Lang component.',
          '',
          '**Hold mode** (default): recognition runs while the button is physically held (pointerdown → pointerup/leave/cancel). Pointercancel and Escape abort without sending.',
          '',
          '**Tap mode**: first click starts, second click (or Escape) stops; Escape aborts.',
          '',
          'On recognition completion the element dispatches a `wiki-page-app-action` event carrying a `continue_conversation` step whose `message` is the template with `{{transcript}}` filled in.',
          '',
          'Degrades gracefully when `SpeechRecognition`/`webkitSpeechRecognition` are absent (e.g. Firefox today): renders a visually disabled button with an accessible tooltip — never throws.',
        ].join('\n'),
      },
    },
  },
  argTypes: {
    label: { control: 'text', description: 'Button label and aria-label.' },
    template: { control: 'text', description: 'Optional message template. `{{transcript}}` is replaced with the recognised text. Absent → raw transcript is sent.' },
    mode: { control: 'radio', options: ['hold', 'tap'], description: '`hold`: record while pressed. `tap`: click to start, click again to stop.' },
    disabled: { control: 'boolean', description: 'Disables the button.' },
    interim: { control: 'boolean', description: 'Show live interim transcript below the button (default true).' },
    lang: { control: 'text', description: 'BCP-47 language tag, e.g. "en-US". Defaults to navigator.language.' },
  },
};

export default meta;
type Story = StoryObj;

export const HoldMode: Story = {
  name: 'Hold mode (default)',
  args: {
    label: 'Hold to Speak',
    mode: 'hold',
    interim: true,
  },
  render: (args) => html`<wiki-page-app-mic-button
    .label=${args['label']}
    .mode=${args['mode']}
    .interim=${args['interim'] !== false}
  ></wiki-page-app-mic-button>`,
};

export const TapMode: Story = {
  name: 'Tap mode (click to toggle)',
  args: {
    label: 'Tap to Speak',
    mode: 'tap',
    interim: true,
  },
  render: (args) => html`<wiki-page-app-mic-button
    .label=${args['label']}
    .mode=${args['mode']}
    .interim=${args['interim'] !== false}
  ></wiki-page-app-mic-button>`,
};

export const WithTemplate: Story = {
  name: 'With message template',
  args: {
    label: 'Add Note',
    template: 'Add a note: {{transcript}}',
    mode: 'hold',
  },
  parameters: {
    docs: {
      description: {
        story: 'Template `"Add a note: {{transcript}}"` — the recognised utterance is inserted at `{{transcript}}` before dispatching the `continue_conversation` message.',
      },
    },
  },
  render: (args) => html`<wiki-page-app-mic-button
    .label=${args['label']}
    .template=${args['template']}
    .mode=${args['mode']}
  ></wiki-page-app-mic-button>`,
};

export const Disabled: Story = {
  args: {
    label: 'Speak',
    disabled: true,
  },
  render: (args) => html`<wiki-page-app-mic-button
    .label=${args['label']}
    .disabled=${args['disabled'] === true}
  ></wiki-page-app-mic-button>`,
};

export const UnsupportedBrowser: Story = {
  name: 'Unsupported browser degradation',
  parameters: {
    docs: {
      description: {
        story: [
          'Simulates Firefox or any browser without `SpeechRecognition`. The element is created with `SpeechRecognition` and `webkitSpeechRecognition` removed from `globalThis` before connection, so `connectedCallback` observes the absent API and sets `speechSupported = false`.',
          '',
          'The button renders disabled with an accessible tooltip and a "Voice input not supported" label — the control is visually present but clearly inoperable.',
        ].join('\n'),
      },
    },
  },
  render: () => {
    const g = globalThis as Record<string, unknown>;
    const savedSR = g['SpeechRecognition'];
    const savedWebkitSR = g['webkitSpeechRecognition'];
    delete g['SpeechRecognition'];
    delete g['webkitSpeechRecognition'];

    const el = document.createElement('wiki-page-app-mic-button') as WikiPageAppMicButton;
    el.label = 'Speak';

    // Restore after the element has connected and detected the missing API
    void el.updateComplete.then(() => {
      if (savedSR !== undefined) g['SpeechRecognition'] = savedSR;
      if (savedWebkitSR !== undefined) g['webkitSpeechRecognition'] = savedWebkitSR;
    });

    return html`${el}`;
  },
};

export const RecordingActive: Story = {
  name: 'Recording active state (animated)',
  parameters: {
    docs: {
      description: {
        story: [
          'Button in the active recording state: `data-listening="true"`. The red gradient replaces the blue, and a **pulsing ring** animates around the circular mic element.',
          '',
          'Under `prefers-reduced-motion: reduce` the animation is suppressed and replaced by a static outline so the state remains visible.',
          '',
          'State is simulated with a fake `SpeechRecognition` that starts but never fires `onend` — the button stays in the recording state.',
        ].join('\n'),
      },
    },
  },
  render: () => {
    const g = globalThis as Record<string, unknown>;
    const savedSR = g['SpeechRecognition'];
    const savedWebkitSR = g['webkitSpeechRecognition'];

    class PersistentFakeSR {
      continuous = false;
      interimResults = false;
      maxAlternatives = 1;
      lang = '';
      onresult: ((event: unknown) => void) | null = null;
      onerror: ((event: unknown) => void) | null = null;
      onend: (() => void) | null = null;
      start(): void { /* intentionally never fires onend */ }
      stop(): void {}
      abort(): void {}
    }

    g['SpeechRecognition'] = PersistentFakeSR;
    delete g['webkitSpeechRecognition'];

    const el = document.createElement('wiki-page-app-mic-button') as WikiPageAppMicButton;
    el.label = 'Hold to Speak';
    el.mode = 'hold';

    void el.updateComplete.then(() => {
      const btn = el.shadowRoot?.querySelector('button');
      btn?.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, composed: true, pointerId: 1 }));
      // Restore globals after the element has started recording
      if (savedSR !== undefined) g['SpeechRecognition'] = savedSR;
      else delete g['SpeechRecognition'];
      if (savedWebkitSR !== undefined) g['webkitSpeechRecognition'] = savedWebkitSR;
    });

    return html`${el}`;
  },
};

export const HoverAndPressStates: Story = {
  name: 'Interactive: hover and press states',
  args: {
    label: 'Hold to Speak',
    mode: 'hold',
  },
  parameters: {
    docs: {
      description: {
        story: [
          'Demonstrates the skeuomorphic **hover** and **press** states.',
          '',
          '- **Hover**: button lifts slightly (`translateY(-1px)`) with enhanced drop-shadow',
          '- **Press/active**: button depresses (`translateY(2px)`) with compressed inset shadows — a tactile push-in effect',
          '',
          'Hover over or press the button to see these states.',
        ].join('\n'),
      },
    },
  },
  render: (args) => html`<wiki-page-app-mic-button
    .label=${args['label']}
    .mode=${args['mode']}
  ></wiki-page-app-mic-button>`,
};

export const InteractiveFiringAction: Story = {
  name: 'Interactive: wiki-page-app-action fired on recognition end',
  args: {
    label: 'Speak',
    mode: 'hold',
    template: 'Transcription: {{transcript}}',
  },
  parameters: {
    docs: {
      description: {
        story: [
          'Hold the button and speak; on release the element dispatches a `wiki-page-app-action` event with a `continue_conversation` step.',
          '',
          'Open the browser developer tools console to see the fired action details and verify the event payload.',
        ].join('\n'),
      },
    },
  },
  render: (args) => {
    const el = document.createElement('wiki-page-app-mic-button') as WikiPageAppMicButton;
    el.label = typeof args['label'] === 'string' ? args['label'] : 'Speak';
    el.mode = args['mode'] === 'tap' ? 'tap' : 'hold';
    el.template = typeof args['template'] === 'string' ? args['template'] : undefined;
    el.addEventListener('wiki-page-app-action', (event) => {
      const detail = (event as CustomEvent).detail;
      console.log('wiki-page-app-action fired:', JSON.stringify(detail, null, 2));
    });
    return html`${el}`;
  },
};
