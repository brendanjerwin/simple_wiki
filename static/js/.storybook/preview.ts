import type { Preview } from '@storybook/web-components-vite'

// Load base styles globally for all stories (same as main application)
import '../../../static/vendor/css/base-min.css'
import '../../../static/vendor/css/menus-min.css'
import '../../../static/vendor/css/fontawesome.min.css'
import '../../../static/vendor/css/solid.min.css'
import '../../../static/vendor/css/dropzone.css'
import '../../../static/vendor/css/github-markdown.css'
import '../../../static/vendor/css/highlight.css'
import '../../../static/css/default.css'

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
       color: /(background|color)$/i,
       date: /Date$/i,
      },
    },
  },
};

export default preview;