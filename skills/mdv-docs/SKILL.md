---
name: mdv-docs
description: Write, update, push to Notion or share by secret link the local Markdown documentation kept in the mdv docs folder. Use whenever documentation, findings, plans, notes or a work summary are written, updated, moved, uploaded to Notion (`mdv notion`) or shared by link (`mdv share`), in any repository.
---

# mdv docs

All documentation lives in one local docs folder, viewed with the `mdv` terminal viewer. Run `mdv --dir` to print its path (default `~/dev/md`, changed with `MDV_DIR`). Don't write docs into the repository you're working in unless the user asks for that.

This file follows the open Agent Skills format (`SKILL.md`). Agents without skill support can read it as plain instructions, for example by referencing it from `AGENTS.md`.

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

- One `# H1` on the first line. `mdv notion push` sends it as the page title. A leading emoji in the H1 becomes the page icon.
- Use only `##` and `###` below it.
- No YAML frontmatter and no raw HTML.
- GFM only: tables, `- [ ]` checklists, fenced code blocks with a language tag, and `> 💡 …` blockquotes for callouts.
- Use absolute URLs. Relative links to other docs break in Notion and in shared gists. Write repo paths as `code`.
- **Images:** put them in a folder named after the doc (`{name}/` next to `{name}.md`) and reference them by relative path, each on its own line: `![caption](name/screenshot.png)`. Both `mdv notion push` and `mdv share` upload local images. Images outside the docs folder are refused.
- Keep paragraphs short. Put key facts (repo, ticket, branch, scope) in a short bold-label list under the H1.
- Emoji help scanning in the H1 and on callouts (✅ done, ⚠️ risk, 💡 tip, 🐛 bug, 📋 plan, 🔍 findings). Don't put one on every heading.

## Before you write anything outside the docs folder

Reading is always fine. **Every Notion write and every share needs the user's explicit approval first.** Tell the user exactly what will happen, then wait.

Run `mdv doctor` if a command fails because of setup. It's read-only and checks the docs folder, Notion access and GitHub access.

## Uploading to Notion (`mdv notion`)

mdv only writes to pages it created, which are recorded in `<docs>/.notion.json`.

1. Ask whether the doc should go to Notion and **where** (which parent page).
2. `mdv notion status` lists docs and their sync state. It's read-only.
3. New pages always get a title. Suggest one based on the H1 (or a readable form of the file name) and let the user change it.
4. Ask for approval with the exact action:
   - "Create a new page **`<title>`** from `<file>` under `<parent>`", or
   - "Replace the content of `<url>`". For updates, run `mdv notion diff <file>` first (read-only) and show the full diff.
5. Once approved:
   - New: `mdv notion push <file> --parent <page URL or ID> --title "<title>" --yes` (or `--private` for a top-level private page)
   - Update: `mdv notion push <file> --yes`
6. Report the page URL.

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
