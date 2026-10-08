const meta = document.querySelector<HTMLMetaElement>('meta[name="simple-wiki-url"]');
if (meta?.content) {
  console.debug('[Simple Wiki Companion] Wiki URL detected:', meta.content);
  browser.runtime.sendMessage({ type: 'WIKI_URL_DETECTED', wikiUrl: meta.content }).catch(() => {
    // Extension context unavailable (e.g. reloaded mid-session) — no UX surface to report to from a content script.
  });
} else {
  console.debug('[Simple Wiki Companion] No wiki meta tag found on this page');
}
