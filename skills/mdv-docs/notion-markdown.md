# 📝 Notion-flavored Markdown reference

mdv sends docs to Notion as **Notion-flavored Markdown** (NFM, `markdown_version: v2`). NFM is CommonMark + GFM (tables, task lists, strikethrough) plus XML-style tags for blocks Markdown doesn't have, such as toggles, callouts and columns.

- **Plain GFM is the default and always works.** Every existing doc is valid NFM.
- **Notion tags are optional.** Use them when a doc really benefits from them, for example a toggle that hides a long log. They look native in Notion. The mdv viewer renders their structure and colors: content of toggles, tabs and columns is indented on a `┊` rail. `mdv share` turns them into GitHub-friendly Markdown. Each element's rendering is listed below.
- **The agent index** (YAML frontmatter at the top of a doc) is never sent: mdv strips it before Notion and gists.
- **No other raw HTML.** Tags that aren't listed here (`<div>`, `<sup>`, `<small>`…) end up as literal text in Notion.

Sources:

- [Notion-flavored Markdown spec](https://developers.notion.com/guides/data-apis/notion-flavored-markdown.md)
- [Migration guide v1 → v2](https://developers.notion.com/guides/data-apis/migrate-to-notion-flavored-markdown.md)

✅ marks syntax confirmed against the live API with mdv. The rest is from the spec.

## Block tag rules

Every block tag follows the same shape:

```md
<toggle>
Title on the first line after the opening tag

Children after a blank line. Not indented.
</toggle>
```

- Put opening and closing tags on their own lines.
- The **title** is the first line after the opening tag. It isn't an attribute.
- **Children** come after a blank line, without indentation, `>` prefixes or `<p>` wrappers.
- Don't add blank lines right after an opening tag or right before a closing tag. They become stray whitespace.
- Leave a blank line between blocks. In v2 a single newline doesn't start a new block.
- Tags nest. For example, a callout can sit inside a toggle.

**Colors** (the `color` attribute):

- text: `gray`, `brown`, `orange`, `yellow`, `green`, `blue`, `purple`, `pink`, `red`
- background: the same names with a `_bg` suffix, such as `blue_bg`

## 1. Recommended in docs

These degrade cleanly. The mdv viewer renders each one, and shared gists get a GitHub-friendly version. Colors need a 256-color terminal.

| Element | Syntax | Children | Viewer fallback | Gist fallback |
|---|---|---|---|---|
| ✅ Toggle (drawer) | `<toggle>` + title line … `</toggle>` | yes | `▸ title`, content indented on a `┊` rail (always open) | `<details><summary>` (collapsible on GitHub) |
| ✅ Toggle heading | `<toggle heading="h1\|h2\|h3\|h4" color="…">` + title … `</toggle>` | yes | heading `▸ title`, content on a rail | `<details>` with a bold summary |
| ✅ Callout | `<callout icon="💡" color="yellow_bg">` + text … `</callout>` | yes | blockquote starting with the icon, first line in the callout's color | blockquote starting with the icon |
| ✅ Columns | `<columns>` `<column ratio="50">` … `</column>` `</columns>` (`ratio` optional, percent) | yes | `◫ Column 1`, `◫ Column 2`… one after another, each with its content on a rail | columns one after another |
| ✅ Tabs | `<tabs>` `<tab icon="📘">` + title … `</tab>` `</tabs>` | yes | each tab title with its content on a rail | each tab as a bold title + content |
| ✅ Colored quote | `<quote color="purple_bg">` + title … `</quote>` | yes | blockquote, title in its color | blockquote |
| Colored paragraph | `<p color="gray_bg">` + text `</p>` | no | paragraph in its color | plain paragraph |
| Colored heading | `<h1 color="blue">` … `<h4>` + title + closing tag | no (children become siblings) | `#`–`####` heading in its color | `#`–`####` heading |
| Colored list item | `<ul>` `<li color="blue">` + text `</li>` `</ul>` | yes | `- item` in its color | `- item` |
| Numbered list type | `<ol type="1\|a\|i">` `<li>` … `</ol>` | yes | `1. item` | same |
| Colored to-do | `<todo checked="true\|false" color="green">` + text `</todo>` | no | `[✓] text` in its color | `- [x] text` |
| Block equation | `<equation color="…">x + y = z</equation>`, or bare `$$` lines around it | no | ` ```math ` block | ` ```math ` block (rendered by GitHub) |
| Styled table | `<table fit-page-width="true" header-row="true" header-column="false">`, optional `<colgroup><col width="180" color="…" /></colgroup>`, then `<tr color>` / `<th>` / `<td color>`; one cell per line, `<br />` for line breaks | cells are inline only | pipe table (first row = header, colors dropped) | same |
| ✅ Table of contents | `<table-of-contents color="…" />` | no | dropped | dropped |
| Empty line | `<p />` | no | blank line | blank line |
| ✅ Colored text | `<span color="orange">text</span>` | inline | text in its color | text only |
| ✅ Underline | `<u>text</u>` (the only underline) | inline | underlined | `<ins>text</ins>` |
| Bold / italic / strike | `<b>` `<i>` `<s>` (the Markdown forms `**`, `*`, `~~` also work) | inline | `**` `*` `~~` | same |
| Inline code | backticks (Notion *returns* `<code>`) | inline | backticks | same |
| ✅ Inline math | `$E=mc^2$` | inline | unchanged | unchanged (GitHub renders it) |
| Line break | `<br />`, or a trailing `\` | inline | line break | `<br>` |
| Date | `<date start="2026-12-01" end="…" start-time="HH:mm" end-time="HH:mm" time-zone="Europe/Stockholm" />` | inline | `2026-12-01 → …` | same |
| Citation | `[^https://example.com]` (no footnotes) | inline | `[[source]](url)` link | same |
| Custom emoji | `:smile:` | inline | unchanged | unchanged |

## 2. Notion-only, with a link fallback

These work, but outside Notion they become plain links.

| Element | Syntax | Viewer and gist fallback |
|---|---|---|
| Mention | `<mention url="https://…">Label</mention>`, `<mention url="…" />`. Targets: `user://`, page URLs, `collection://`, `agent://`, `group://`; add `type="database"` for databases | `[Label](url)` for web URLs, `@Label` for users |
| Bookmark | `<bookmark source="https://…" />` | `[bookmark](url)` |
| Embed | `<embed source="https://…" />` | `[embed](url)` |
| File, PDF, audio, video | `<file source="…">Caption</file>`, `<pdf source="…" />`, `<audio source="…" />`, `<video source="…">Caption</video>` | `[caption](url)` |
| Image (XML form) | `<image source="…" color="…">caption</image>` | `![caption](url)`. Without a source (Notion's read-back of a placeholder): `🖼 caption` in the viewer |

> 💡 **For local screenshots, keep using `![caption](doc-name/screenshot.png)`.** mdv uploads the file and places it in Notion. The XML image form is only for images that are already online.

## 3. Don't use in local docs

These need existing Notion IDs, are read-only, or can cause damage. mdv leaves them as they are and shows them as raw text.

| Element | Why |
|---|---|
| `<page url="…">Title</page>`, `<folder>`, `<database …>` | Point at existing pages. **Leaving out a `<page>` tag on update can delete that subpage.** mdv never sets `allow_deleting_content`, and per the spec the update then fails instead. |
| `<synced-block>`, `<synced-block-reference>` | Need IDs of existing synced blocks. A new reference overwrites the original everywhere it appears. |
| `<meeting-notes>` and its sections | Notion meeting notes. The transcript is read-only. |
| `<custom-block>`, `<external_object_instance>`, `<mail>` | Integration-specific. |
| `<link-preview>` | Read-only marker. It can't be created. |
| `<details>` / `<summary>` | Legacy. ✅ Notion converts it to `<toggle>` with a `deprecated_tag` warning. Write `<toggle>` instead. |

## 4. Plain Markdown rules that changed in v2

- **Blank line between blocks.** A single newline doesn't start a new block, and two quotes with no blank line between them merge.
- **Quotes:** leave a blank line after a `>` quote, otherwise the next line joins it.
- **Nested lists:** indent with spaces, 2 under `-` and 3 under `1.`. Tabs also work for lists.
- **Images go on their own line,** with blank lines around them. An image inside a sentence is a parse error.
- **Code must be fenced,** never indented. Use a longer outer fence (````) when the content contains ```.
- **Escape literal tags** as `\<tag>`, for example `\<today>`. ✅ An unknown `<tag>` in text is kept as literal text but triggers an `unsupported_tag` warning.
- **Block math:** use `$$` on its own lines. One-line `$$…$$` and `\[…\]` are invalid.
- **No footnotes.** Use citations (`[^URL]`) or links.
- **Headings:** `#` to `####` only.
- **Parse errors fail the push** with `Markdown errors blocked parsing`. Nothing gets saved halfway.

## Warnings from Notion

`mdv notion push` prints any warnings Notion returns, for example:

```text
⚠️ Notion: unsupported_tag: <today> is not a supported tag and was applied as raw text. Escape it as \<today> if literal text was intended.
```

| Code | Meaning | What to do |
|---|---|---|
| `unsupported_tag` | A tag Notion doesn't know was saved as text | Escape it (`\<tag>`) or use a supported tag |
| `deprecated_tag` | A v1 or legacy tag was converted | Switch to the v2 tag it names |
| `html_tag_autofix` | An HTML tag was removed or converted | Remove the HTML |

## Known gaps in Notion's documentation

- The spec says `<details>` is saved as text, but the API converts it to a toggle (confirmed). Don't rely on either behaviour.
- Bookmark and embed are listed as supported, but one API page says they're "not yet supported" on read.
- Notion returns inline code as `<code>`, but the spec only lists `<b> <i> <s> <u>`. Write backticks.
- Headings `#####` and deeper aren't documented.
- Callout, tab and mail attributes aren't fully documented. Stick to the forms above.
