import type { TemplateResult } from 'lit';
import { render } from 'lit';
import { expect } from '@open-wc/testing';
import * as layoutStories from './layout.stories.js';
import * as checkboxStories from './checkbox.stories.js';
import * as radioStories from './radio.stories.js';
import * as sliderStories from './slider.stories.js';
import * as numberinputStories from './numberinput.stories.js';
import * as textareaStories from './textarea.stories.js';
import * as datepickerStories from './datepicker.stories.js';
import * as badgeStories from './badge.stories.js';
import * as iconStories from './icon.stories.js';
import * as avatarStories from './avatar.stories.js';
import * as skeletonStories from './skeleton.stories.js';
import * as listStories from './list.stories.js';
import * as imageStories from './image.stories.js';
import './page-app-story-wrap.js';
import type { PageAppStoryWrap } from './page-app-story-wrap.js';

/**
 * Smoke tests over the wiki-core@3 Storybook stories: every story program
 * parses through the real wiki library and renders without surfacing the
 * structured "parse errors" block — the same bar a Storybook page must meet.
 */

/** A named export that is a Storybook story: a render callback into a template. */
interface StoryLike {
 name: string;
 render: () => TemplateResult;
}

/** Narrow a module export into a story: objects carrying a render callback. */
function isStory(value: unknown): value is StoryLike {
 return typeof value === 'object'
  && value !== null
  && 'render' in value
  && typeof value.render === 'function';
}

type StoryModule = Record<string, unknown>;

const STORY_MODULES: StoryModule[] = [
 layoutStories as StoryModule,
 checkboxStories as StoryModule,
 radioStories as StoryModule,
 sliderStories as StoryModule,
 numberinputStories as StoryModule,
 textareaStories as StoryModule,
 datepickerStories as StoryModule,
 badgeStories as StoryModule,
 iconStories as StoryModule,
 avatarStories as StoryModule,
 skeletonStories as StoryModule,
 listStories as StoryModule,
 imageStories as StoryModule,
];

function storyExports(module: StoryModule): StoryLike[] {
 const stories: StoryLike[] = [];
 for (const [name, value] of Object.entries(module)) {
  if (isStory(value)) {
   stories.push({ name, render: () => value.render() });
  }
 }
 return stories;
}

async function awaitWrappers(root: HTMLElement): Promise<unknown> {
 const wrappers = Array.from(root.querySelectorAll<PageAppStoryWrap>('wiki-page-app-story-wrap'));
 return Promise.all(wrappers.map((el) => el.updateComplete));
}

/** The first parse-error block found under the wrappers, if any. */
function parseErrorsIn(root: HTMLElement): string | null {
 let seen: string | null = null;
 for (const wrapper of root.querySelectorAll<PageAppStoryWrap>('wiki-page-app-story-wrap')) {
  const pre = wrapper.shadowRoot?.querySelector('pre');
  if (pre?.textContent && pre.textContent.startsWith('parse errors:')) {
   seen = pre.textContent;
  }
 }
 return seen;
}

describe('wiki-core@3 storybook story programs', () => {
 for (const module of STORY_MODULES) {
  for (const story of storyExports(module)) {
   // ParseErrorTransparency's whole contract is showing the parse-error
   // block; asserted separately below.
   if (module === layoutStories && story.name === 'ParseErrorTransparency') {
    continue;
   }
   it(`renders without parse errors: ${story.name}`, async () => {
    const div = document.createElement('div');
    document.body.appendChild(div);
    render(story.render(), div);
    await awaitWrappers(div);
    try {
     const errors = parseErrorsIn(div);
     expect(errors, `${story.name} rendered parse errors: ${errors ?? ''}`).to.equal(null);
    } finally {
     render(null, div);
     div.remove();
    }
   });
  }
 }

 it('layout ParseErrorTransparency surfaces its parse-error block by contract', async () => {
  const story = storyExports(layoutStories).find((entry) => entry.name === 'ParseErrorTransparency');
  expect(story, 'ParseErrorTransparency story is missing').to.exist;
  if (!story) return;

  const div = document.createElement('div');
  document.body.appendChild(div);
  render(story.render(), div);
  await awaitWrappers(div);
  try {
   const errors = parseErrorsIn(div);
   expect(errors, 'the story should surface a parse-error block').to.be.a('string');
   expect(errors, parseErrorsIn(div) ?? '').to.contain('parse errors:');
  } finally {
   render(null, div);
   div.remove();
  }
 });
});
