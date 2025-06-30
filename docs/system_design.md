# System Design

When a page is saved, its contents are saved to a .json file acting as a database of history for that page. The current version of the page is saved as a .md file.

The filename used is a hash of a "munged" version of the page name. "munging" helps common equivalency expectations be met. i.e. "Home" and "home" are equivalent.

Frontmatter in the markdown is used to store structured data that can be referenced by the templating system. This provides a means for slightly more dynamic content. (of particular note: this capability is at the core of the inventory usage pattern).

> **Note:** Frontmatter values are limited to strings, lists of strings, or nested maps. Other data types like numbers or booleans are not directly supported and should be stored as strings if needed.

> _simple_wiki_ is flexible when reading your pages—it understands both `---` (YAML) and `+++` (TOML) frontmatter, and it's smart enough to figure out what you meant even if you mix them up. When you save a page, however, it standardizes the format, gently nudging everything towards the obviously superior TOML `+++` format. This keeps things tidy under the hood.

When a page is saved, it is indexed for later searching. This index is also used to power some of the templating functions allowing for some structured data querying.
