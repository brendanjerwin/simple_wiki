import type { Preview } from '@storybook/web-components-vite'

// Load FontAwesome styles globally for all stories
import '../../../static/vendor/css/fontawesome.min.css'
import '../../../static/vendor/css/solid.min.css'

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