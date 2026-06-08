+++
identifier = "help_macro_map_embed"

[wiki]
system = true
+++

#help #macros

# {{.Title}}

The MapEmbed macro renders a responsive Google Maps embed iframe from a Google Maps embed URL.

## Syntax

```
{{ MapEmbed "https://www.google.com/maps/embed?pb=..." }}
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| First | string | A Google Maps embed URL — must start with `https://www.google.com/maps/embed` |

### Example

```
{{ MapEmbed "https://www.google.com/maps/embed?pb=!1m18!1m12!1m3!1d..." }}
```

## How to Get a Google Maps Embed URL

1. Open [Google Maps](https://maps.google.com) and navigate to the location, route, or map you want to embed.
2. Click **Share** → **Embed a map**.
3. Copy the URL from the `src="..."` attribute of the iframe code Google provides.
4. Paste the URL as the argument to `MapEmbed`.

> [!NOTE]
> Copy only the URL itself (the value of `src="..."`), not the full `<iframe>` HTML tag.

## Safety

Only URLs beginning with `https://www.google.com/maps/embed` are accepted. Any other URL renders an error placeholder instead. This prevents the macro from being used to embed arbitrary third-party content.

## Responsive Layout

The map is wrapped in a responsive container that maintains a 16:9 aspect ratio across all screen sizes, so it scales correctly on both desktop and mobile devices.

## See Also

- [[help-templating]] — full list of available macros
