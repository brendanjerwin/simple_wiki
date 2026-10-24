import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
// Side-effect import: registers wiki-page-app-countdown (customElements.define
// lives in library.ts; the type-only import below is erased at runtime).
import '../openui-lang/library.js';
import type { WikiPageAppCountdown } from '../openui-lang/library.js';

const meta: Meta = {
  title: 'Components/WikiPageAppCountdown',
  component: 'wiki-page-app-countdown',
  parameters: {
    layout: 'centered',
    docs: {
      description: {
        component: 'The ticking element behind the wiki library\'s `Countdown(seconds, label?, onComplete?, running?)` OpenUI Lang component. IDLE until `running` is true; ticks down once per second, renders the remaining time as m:ss (h:mm:ss past an hour). `running` flipping false pauses; a fresh true after finish restarts from full duration. On completion fires `onComplete` as a `wiki-page-app-action` event — executed by the page app host just like a Button action.'
      },
    },
  },
  argTypes: {
    seconds: { control: 'number', description: 'Duration in seconds; a change refills the clock without starting.' },
    label: { control: 'text', description: 'Optional label shown above the ticking readout.' },
    running: { control: 'boolean', description: 'Run gate: true starts/resumes, false pauses, fresh true after finish restarts.' },
  },
};

export default meta;
type Story = StoryObj;

export const Ticking: Story = {
  name: 'Ticking (30s hold)',
  args: {
    seconds: 30,
    label: 'hold',
    running: true,
  },
  render: (args) => html`<wiki-page-app-countdown .seconds=${args['seconds']} .label=${args['label']} .running=${args['running']}></wiki-page-app-countdown>`,
};

export const LongDuration: Story = {
  name: 'Long duration (h:mm:ss)',
  args: {
    seconds: 600,
    label: 'warm-up',
    running: true,
  },
  render: (args) => html`<wiki-page-app-countdown .seconds=${args['seconds']} .label=${args['label']} .running=${args['running']}></wiki-page-app-countdown>`,
};

export const Idle: Story = {
  name: 'Idle (default — page load must not start)',
  args: {
    seconds: 45,
  },
  render: (args) => html`<wiki-page-app-countdown .seconds=${args['seconds']}></wiki-page-app-countdown>`,
};

export const PausedMidRun: Story = {
  name: 'Paused mid-run',
  render: () => {
    const el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
    el.seconds = 30;
    el.label = 'hold';
    // Simulate a paused run frozen at 17s: mutate only after first
    // update — willUpdate resets state props on the initial seconds commit.
    void el.updateComplete.then(() => {
      (el as unknown as { remainingSeconds: number }).remainingSeconds = 17;
    });
    return html`${el}`;
  },
};

export const FinishedState: Story = {
  name: 'Finished state (readout highlighted, completion announced)',
  render: () => {
    const el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
    el.seconds = 30;
    el.label = 'hold';
    void el.updateComplete.then(() => {
      (el as unknown as { remainingSeconds: number }).remainingSeconds = 0;
      (el as unknown as { finished: boolean }).finished = true;
    });
    return html`${el}`;
  },
};

export const CompleteFiresAction: Story = {
  name: 'Interactive: onComplete fires wiki-page-app-action',
  args: {
    running: false,
  },
  parameters: {
    docs: {
      description: {
        story: 'A 5-second countdown with an onComplete plan (@Set($holds, $holds + 1) semantics — passed here as a raw plan object). Flip the `running` control to true (or toggle it off/on after a finish to restart). When it hits zero, the element dispatches the wiki-page-app-action event. Open the browser developer tools console to see the fired action details and verify the event payload.'
      },
    },
  },
  render: (args) => {
    const el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
    el.seconds = 5;
    el.label = 'hold';
    el.onComplete = { steps: [{ type: 'set', target: '$holds', valueAST: { k: 'Num', n: 1 } }] };
    el.running = args['running'] === true;
    el.addEventListener('wiki-page-app-action', (event) => {
      const detail = (event as CustomEvent).detail;
      console.log('wiki-page-app-action fired:', JSON.stringify(detail, null, 2));
    });
    return html`${el}`;
  },
};