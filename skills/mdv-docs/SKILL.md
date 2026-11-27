---
name: mdv-docs
description: Find, write, update, push to Notion or share by secret link the local Markdown documentation kept in the mdv docs folder. Use before starting work on a ticket or change (to find earlier docs with `mdv find`), and whenever documentation, findings, plans, notes or a work summary are written, updated, moved, uploaded to Notion (`mdv notion`) or shared by link (`mdv share`), in any repository.
---

# mdv docs

All documentation lives in one local docs folder, viewed with the `mdv` terminal viewer. Run `mdv --dir` to print its path (default `~/dev/md`, changed with `MDV_DIR`). Don't write docs into the repository you're working in unless the user asks for that.

This file follows the open Agent Skills format (`SKILL.md`). Agents without skill support can read it as plain instructions, for example by referencing it from `AGENTS.md`. `mdv skill diff` shows whether this copy matches the installed mdv version.

## Before you start work: find earlier docs

Earlier docs often already explain the code, decisions and pitfalls behind a new task. Before you start work on a ticket or a change in a repo, search the docs folder:

```sh
mdv find --json --ticket TAP-1234
mdv find --json --repo web-monorepo fetch headers
```

- Search with the ticket, the repo (and app, `--app`), and 2–4 key terms from the task. Use `--path` for a folder or file you'll change.
- `mdv find` is read-only. Results are ranked, say why they matched, and include each doc's agent index: summary, status, PR and caveats. Docs without an index are matched by text, folder and file name.
- Read the summary and caveats of the top matches, and open the docs that are relevant.
- Tell the user which earlier docs you found and how they affect the plan, for example "TAP-4968 notes that interceptors moved to ofetch hooks".

## Agent index (every new doc)

Every new doc starts with an **agent index**: YAML frontmatter that lets agents find and understand the doc without reading all of it. mdv strips it before Notion and gists, shows it as a header in the viewer, and ignores it when deciding whether a doc needs re-pushing.

```yaml
---
ticket: TAP-1234                 # if there is one
repo: web-monorepo               # required: repo folder name
apps: [web-app]                  # monorepo apps or packages involved
type: investigation              # required: investigation | plan | summary | how-to | reference | bug
status: in-progress              # required: in-progress | done | abandoned | superseded
summary: >                       # required: 1–3 sentences, what was done or found, and how
  Replaced axios with Nuxt $fetch in web-app; api-sdk keeps its HttpAdapter.
keywords: [axios, fetch, ofetch] # required: terms someone would search for
paths: [packages/http]           # key files or folders changed or studied
branch: tap-1234-remove-axios
pr: 1787                         # number or URL, once there is one
caveats:                         # what future work must know
  - Interceptors moved to ofetch hooks (onRequest, onResponseError)
related: [TAP-4398]              # tickets or doc paths
created: 2026-11-27
updated: 2026-11-27              # required
---
# 🔍 TAP-1234: Remove axios from web-app
```

- The summary says **what** and **how**, not just the topic.
- Caveats are concrete: pitfalls, workarounds, things that broke, decisions others must respect.
- Keep it current. When work progresses, update `status`, `pr`, `branch` and `updated`, for example after the PR is merged.
- Never put secrets, credentials or customer data in it.
- Add an index to every new doc. Don't add one to existing docs unless the user asks. `mdv find --unindexed` lists docs without one.
- When a newer doc replaces this one, set `status: superseded` and `superseded_by: <doc path>`.

## Starting and checking a doc

Every doc type has a fixed set of `##` sections, in order, so later agents can jump straight to "Caveats" or "For future work". The types are `investigation`, `plan`, `summary`, `bug`, `how-to` and `reference`. [templates.md](templates.md), next to this file, lists each type's sections and what goes in them, plus how to write for the next agent. **Read it before writing a doc.**

1. **Start** a new doc with `mdv new <type> [TICKET] <slug>`, run from inside the repo (and app) the work is about. It writes the skeleton with the agent index filled in (repo, app, branch, dates) and never overwrites an existing doc. Use `--repo`/`--app` when you're not in the repo.
2. **Fill it in.**
   - Write the summary and keywords.
   - Write each section with concrete anchors: paths, names, commands, links.
   - Give decisions with their reasons.
   - Make the state explicit (write "None." for empty sections in a done doc).
   - No wording like "yesterday" or "as discussed".
3. **Check** with `mdv check <file>`. It's read-only, exits with 1 on errors, and lists warnings. Fix the errors before you report the doc as done, and fix the warnings unless there's a reason not to. `mdv notion push` and `mdv share` also warn when a doc has issues.

## Where files go

| Situation | Path inside the docs folder |
|---|---|
| Single-app repo | `{repo}/{name}.md` |
| Monorepo, one app | `{repo}/{app}/{name}.md` |
| Monorepo, several apps or the whole repo | `{repo}/{name}.md` |
| Not about one repo | Ask the user where it should go |

- `{repo}` is the repository folder name. `{app}` is the folder under `apps/` (or `packages/`) in a monorepo.
- Name files `TICKET-123-kebab-slug.md` when there is a ticket, and `kebab-slug.md` otherwise.
- If a doc for this work already exists, update it instead of creating a new one.
- Never edit documentation files you didn't create unless the user asks.

## Notion-ready format (all new files)

- One `# H1` on the first line after the agent index. `mdv notion push` sends it as the page title. A leading emoji in the H1 becomes the page icon.
- Use only `##` and `###` below it.
- No YAML frontmatter other than the agent index, and no raw HTML except the Notion tags below.
- GFM: tables, `- [ ]` checklists, fenced code blocks with a language tag, and `> 💡 …` blockquotes for callouts.
- Use absolute URLs. Relative links to other docs break in Notion and in shared gists. Write repo paths as `code`.
- **Images:** put them in a folder named after the doc (`{name}/` next to `{name}.md`) and reference them by relative path, each on its own line: `![caption](name/screenshot.png)`. Both `mdv notion push` and `mdv share` upload local images. Images outside the docs folder are refused.
- Keep paragraphs short. Put key facts (repo, ticket, branch, scope) in a short bold-label list under the H1.
- Emoji help scanning in the H1 and on callouts (✅ done, ⚠️ risk, 💡 tip, 🐛 bug, 📋 plan, 🔍 findings). Don't put one on every heading.

### Two styles: plain GFM and Notion tags

mdv sends docs to Notion as Notion-flavored Markdown, which is GFM plus XML-style tags. **Plain GFM is the default and always works.** Notion tags are optional. Use them only when they clearly help, for example a `<toggle>` (drawer) around a long log. They look native only in Notion. The mdv viewer and `mdv share` show a readable fallback.

The full list of elements, syntax and fallbacks is in [notion-markdown.md](notion-markdown.md), next to this file. Read it before using any tag. In short:

1. **Recommended:**
   - `<toggle>`, `<toggle heading="h2">`
   - `<callout icon color>`
   - `<columns>`/`<column>`, `<tabs>`/`<tab>`
   - colored `<quote>`, `<p>`, `<h1>`–`<h4>`, `<li>`, `<todo>`
   - `<span color>`, `<u>`
   - `<equation>`, `$…$`
   - styled `<table>`, `<table-of-contents />`, `<date />`
2. **Notion-only, fallback is a link:** `<mention>`, `<bookmark>`, `<embed>`, `<file>`, `<pdf>`, `<audio>`, `<video>`, `<image source>`.
3. **Never in local docs:** `<page>`, `<folder>`, `<database>`, synced blocks, meeting notes, custom blocks, `<mail>`, `<link-preview>`, `<details>`.
4. **Rules that apply to every doc:**
   - Blank line between blocks, and after a `>` quote.
   - Images on their own line.
   - Code always fenced.
   - Nested lists indented with spaces.
   - Escape literal tags in text as `\<tag>`.

## Before you write anything outside the docs folder

Reading is always fine. **Every Notion write and every share needs the user's explicit approval first.** Tell the user exactly what will happen, then wait. The user's agent may also be set up to prompt for every `mdv … --yes` command. That's expected: it's the second half of the approval, not an error.

Only use `mdv` for Notion and gists. **Never** call `ntn`, `gh gist`, or the Notion or GitHub APIs directly, not even to read. `mdv notion schema` and `mdv notion info` cover the reads.

Run `mdv doctor` if a command fails because of setup. It's read-only and checks the docs folder, Notion access and GitHub access.

## Uploading to Notion (`mdv notion`)

mdv only writes to pages it created, which are recorded in `<docs>/.notion.json`.

1. Ask whether the doc should go to Notion and **where**: a parent page or a database. A new page gets the access of where it's created. `--private` pages are visible only to the user, and mdv can't change sharing, so for a doc others should read, suggest a shared page or database.
2. `mdv notion status` lists docs and their sync state. It's read-only.
3. New pages always get a title. Suggest one based on the H1 (or a readable form of the file name) and let the user change it.
4. Ask for approval with the exact action:
   - "Create a new page **`<title>`** from `<file>` under `<parent>`", or
   - "Create a new entry **`<title>`** from `<file>` in the database `<database>` with `<Name=value; …>`", or
   - "Replace the content of `<url>`". For updates, run `mdv notion diff <file>` first (read-only) and show the full diff.
5. Once approved:
   - New: `mdv notion push <file> --parent <page URL or ID> --title "<title>" --yes` (or `--private` for a top-level private page)
   - New database entry: `mdv notion push <file> --database <database URL or ID> --title "<title>" --prop "Name=value" … --yes`
   - Update: `mdv notion push <file> --yes`. Add `--prop "Name=value"` to change a property of a database entry mdv created.
6. Report the page URL, and any `⚠️ Notion:` warnings mdv printed (see [notion-markdown.md](notion-markdown.md#warnings-from-notion)).

To check a parent page or a page link before asking, run `mdv notion info <page URL or ID>`. It's read-only and shows the title, parent and whether mdv created the page.

**Database properties (`--prop`):** before asking, run `mdv notion schema <database URL or ID>`. It's read-only. It lists every property, its options, and whether `--prop` can set it. Propose values from the existing options, and let the user confirm or change them. mdv refuses new options and two-way relations. Don't try to get around that: tell the user to set those by hand in Notion. Never create or change databases.

If mdv refuses because a page was edited in Notion since the last push, stop and tell the user. Don't work around it. Don't edit `.notion.json` by hand, and don't write to Notion with other tools.

## Sharing by secret link (`mdv share`)

`mdv share` publishes a doc as a **secret GitHub gist**: anyone with the link can read it, it isn't listed or searchable, and the local doc never changes. Use it when the user wants to share a doc **without** Notion. If the user just says "share", ask whether they mean Notion or a link.

1. `mdv share status` lists shared docs (✓ shared / ● changed). It's read-only.
2. Ask for approval with the exact action:
   - "Share `<file>` as a secret gist (anyone with the link can read it)", or
   - "Update the shared gist `<url>` with `<file>`" (same link), or
   - "Delete the gist `<url>`" (the link stops working for everyone).
3. Once approved, run `mdv share <file> --yes` or `mdv share rm <file> --yes`.
4. Report the link.

A gist is unlisted, not access-controlled. Warn the user before sharing anything that looks like it contains secrets, credentials or personal/customer data. Don't create or delete gists with other tools, and don't edit `.share.json` by hand.
