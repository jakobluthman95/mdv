# mdv

A documentation workspace for AI coding agents. Your agent writes its findings, plans and summaries to one local folder, you read and review them in the terminal, and the agent publishes them to Notion or a secret link, but only once you approve.

## What and why

Coding agents produce a lot of useful writing: investigation notes, plans, findings, summaries of the work done. Without a convention, it ends up scattered in repos, chat scrollback or `LOCAL_DOCS/` folders that never get shared.

mdv gives agents (and you) one place and one workflow for it:

1. **Write:** the [mdv-docs skill](#ai-agents) tells the agent where docs go (one folder outside your repos, default `~/dev/md`, organised by repo) and how to format them so they publish cleanly.
2. **Review:** `mdv` opens a terminal viewer over every doc, newest first, so you can read what the agent wrote without leaving the terminal.
3. **Publish, with approval:** the agent can push a doc to Notion (`mdv notion`) or turn it into a secret link (`mdv share`). The skill makes it ask first and show you exactly what will change.
4. **Find, before new work:** every new doc starts with a short [agent index](#agent-index-and-mdv-find) (ticket, repo, summary, caveats, PR…). Before starting a ticket, the agent runs `mdv find` to see what earlier docs already cover, so it doesn't rediscover the same code and pitfalls.

The commands are built so an agent can use them safely:

- **Read-only checks first:** `status`, `diff`, `schema`, `info`, `find`, `check` and `doctor` never write anything.
- **Approval for every write:** in a terminal, every write asks `[y/N]`. Agents have no terminal, so they have to pass `--yes`. That's why the skill makes them ask you first, and why you should also [let your agent harness enforce it](#agent-safety).
- **mdv only touches what it created:** it updates only the Notion pages and gists it made itself, recorded in `.notion.json` and `.share.json`.
- **No silent overwrites:** it refuses to overwrite edits someone made in Notion.
- **Local files stay the source of truth:** mdv never edits your docs.

You can use all of it by hand too. The agent is just the main user.

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/jakobluthman95/mdv/main/install.sh | sh
mdv init
mdv skill install
mdv doctor
```

That's it: install, set up, teach your agent, check. The sections below explain each step and the alternatives.

## Requirements

- **macOS or Linux** (Intel or ARM).
- **`git`**: `mdv share` uses it to upload images, and `mdv notion diff` and updates use it to show diffs.
- **To publish:** a Notion **integration token** *or* the `ntn` CLI for `mdv notion`, and a GitHub **token** *or* the `gh` CLI for `mdv share`. See [Configuration](#configuration).

Browsing docs needs only the binary. Building from source also needs Go 1.27+ and `make`.

## Setup, step by step

**1. Install mdv.** Pick one:

- **Install script (recommended):** downloads the right prebuilt binary for your machine from the [latest release](https://github.com/jakobluthman95/mdv/releases/latest), checks its checksum and puts it in `~/.local/bin`:

  ```sh
  curl -fsSL https://raw.githubusercontent.com/jakobluthman95/mdv/main/install.sh | sh
  ```

  `MDV_VERSION=v0.1.0` installs a specific version, and `MDV_INSTALL_DIR=…` installs somewhere else. You can [read the script](install.sh) first.

- **With Go:** installs into `~/go/bin` (or `$GOBIN`):

  ```sh
  go install github.com/jakobluthman95/mdv@latest
  ```

- **From source:** for working on mdv itself:

  ```sh
  git clone https://github.com/jakobluthman95/mdv.git ~/dev/mdv
  cd ~/dev/mdv && make install     # builds ~/.local/bin/mdv
  ```

  You can also download an archive by hand from [Releases](https://github.com/jakobluthman95/mdv/releases).

`mdv --version` shows what you have. The docs folder (`~/dev/md`) and a welcome doc are created the first time you run `mdv`.

**2. Make sure the install folder is on your `PATH`.** The install script tells you if it isn't. For `~/.local/bin`:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

**3. Run the setup:**

```sh
mdv init
```

It asks for the docs folder, a Notion token and a GitHub token, and checks each token right away. Every question can be skipped: without tokens, mdv uses the `ntn` and `gh` CLIs instead (see [Configuration](#configuration) for how to get either). It writes `~/.config/mdv/.env`, readable only by you. Run it again any time to change a value. `mdv init --edit` opens the file in your editor instead.

**4. Give your agent the skill.** This is the step that makes mdv useful to an agent:

```sh
mdv skill install
```

The skill is built into the mdv binary, so it always matches your mdv version. This installs it for Claude Code (`~/.claude/skills/mdv-docs`). Use `--dir` for another agent's skills folder. To install it into several agents at once, use the skills CLI instead. See [AI agents](#ai-agents).

**5. Set up Notion and/or GitHub access** if you skipped the tokens in step 3 and want the agent to publish with `mdv notion` or `mdv share`. See [Configuration](#configuration).

**6. Check everything:**

```sh
mdv doctor
```

`doctor` is read-only. It checks the config file, the docs folder, Notion access, GitHub access and the installed skill, and tells you what's missing.

### Updating

Run the same install command again, then update the skill:

```sh
curl -fsSL https://raw.githubusercontent.com/jakobluthman95/mdv/main/install.sh | sh   # or: go install …@latest, or git pull && make install
mdv skill install
```

`mdv skill install` updates a skill it installed earlier to the new version. If you changed the skill since, it refuses and shows you `mdv skill diff` instead. If you installed it with the skills CLI, run `pnpx skills update` instead.

Your settings in `~/.config/mdv/.env` and your docs are never touched by an update.

## AI agents

The skill is what turns mdv from a viewer into an agent workflow. [`skills/mdv-docs/SKILL.md`](skills/mdv-docs/SKILL.md) tells the agent:

- to search earlier docs with `mdv find` before starting work on a ticket or change;
- where docs go and how to name them, and to start every new doc with an agent index;
- the Notion-ready Markdown format (H1 title, GFM plus optional Notion tags, images next to the doc);
- how to push to Notion and share by link, using the read-only commands first;
- to **always ask before any write**, stating exactly what will happen.

It uses the open `SKILL.md` format and reads fine as plain Markdown. The full Notion element reference, [`notion-markdown.md`](skills/mdv-docs/notion-markdown.md), sits next to it and is installed with it.

Once it's installed, just talk to your agent: "write up these findings", "push that doc to Notion under the team page", "give me a link I can send to a colleague".

### Install with mdv (recommended)

Every mdv binary carries the skill for its own version, so the two always match:

```sh
mdv skill install   # into ~/.claude/skills/mdv-docs (--dir for another agent's skills folder)
mdv skill diff      # compare the installed skill with this mdv version's
mdv skill print     # print it, e.g. to paste into AGENTS.md
```

- **After upgrading mdv,** run `mdv skill install` again to update the skill. mdv keeps a small `.mdv-skill.json` next to the skill to know what it installed.
- **A skill you changed is never overwritten silently.** This includes a skill mdv didn't install. `install` refuses and points you to `mdv skill diff`. `--force` replaces it.
- `mdv doctor` tells you whether the installed skill is up to date, outdated or customised.

### Install with the skills CLI (several agents)

```sh
pnpx skills add https://github.com/jakobluthman95/mdv --skill mdv-docs --global
```

Use this to install the skill into every coding agent you use at once. It uses the open [`skills`](https://github.com/vercel-labs/skills) installer (`npx skills …` works too) and needs Node:

- It clones the public repo over HTTPS. No SSH key is needed.
- It finds `skills/mdv-docs/SKILL.md` and installs it for every coding agent it detects (Claude Code, Codex, Cursor, OpenCode and more).
- `--global` installs it for your user, so it works in all projects. Leave it out to install into the current project only.

To keep it up to date:

```sh
pnpx skills check     # any updates?
pnpx skills update    # install them
```

Run `pnpx skills add https://github.com/jakobluthman95/mdv --list` to see what's in the repo without installing anything.

### Install by hand

- **Agents with skill folders:** copy `skills/mdv-docs` into the agent's skills directory, e.g. `mkdir -p ~/.claude/skills && cp -r skills/mdv-docs ~/.claude/skills/` for Claude Code. Re-copy it after `git pull` if it changed.
- **Anything else:** reference it from your global `AGENTS.md`, or paste it into your agent's instructions.

> 💡 If you already have a skill named `mdv-docs` in an agent's skills folder, remove it or skip that agent first. Otherwise the two will clash.

### Agent safety

The skill tells the agent to ask before every write, but that's an instruction, not a guarantee. The `[y/N]` prompt doesn't help either, because agents have no terminal and must pass `--yes`. To make approval something the agent can't skip, let your agent harness ask you before any mdv write.

**Claude Code:** add this to `~/.claude/settings.json`, merging with any `permissions` you already have:

```json
{
  "permissions": {
    "ask": [
      "Bash(mdv *--yes*)",
      "Bash(mdv notion push *)",
      "Bash(mdv share rm *)",
      "Bash(ntn *)",
      "Bash(gh gist *)"
    ]
  }
}
```

- Every mdv write from an agent needs `--yes`, so you get a prompt showing the exact command. Read-only commands (`status`, `diff`, `schema`, `info`, `doctor`) run without prompts.
- The `ntn` and `gh gist` rules catch an agent that tries to get around mdv.

Permission rules match how a command is written, so `./mdv …`, `~/.local/bin/mdv …` or `cd x && mdv …` can slip past them. For a guard that catches those too, add the hook in [`contrib/claude-code/mdv-guard.sh`](contrib/claude-code/mdv-guard.sh). It forces a prompt with a clear reason for any mdv call with `--yes`, and for any `ntn` or `gh gist` call. It needs `jq`.

```sh
mkdir -p ~/.claude/hooks && cp contrib/claude-code/mdv-guard.sh ~/.claude/hooks/ && chmod +x ~/.claude/hooks/mdv-guard.sh
```

```json
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [{ "type": "command", "command": "~/.claude/hooks/mdv-guard.sh" }] }
    ]
  }
}
```

Merge that into `~/.claude/settings.json` and keep the `ask` rules above as a second layer. Run `/hooks` once, or restart Claude Code, so it picks up the hook.

**Other agents:** use their equivalent, for example a command allowlist or an "ask before running" rule for the same commands.

| The agent may | Approval |
|---|---|
| Write and update docs in the docs folder | Your normal doc rules |
| Run read-only commands | None |
| Create pages, or update pages and gists mdv created | **Required** |
| Touch anything mdv didn't create, call `ntn` / `gh gist` / APIs directly, or edit `.notion.json` / `.share.json` | Never |

## Usage

### Viewing

```sh
mdv                  # browse all docs in the docs folder (recursive, newest first)
mdv path/to/doc.md   # open one file (relative, absolute or ~ path)
mdv --dir            # print the docs folder
mdv --help           # all commands and keys
```

| Keys | Action |
|---|---|
| `↑/↓` `j/k`, mouse wheel | scroll / move |
| `space` `b`, `PgUp/PgDn` | page |
| `u/d` | half page |
| `g/G` | top / bottom |
| `/` | filter the list |
| `enter` | open file |
| `r` | reload file |
| `esc`, `backspace` | back to the list |
| `q`, `ctrl+c` | quit |

A doc's [agent index](#agent-index-and-mdv-find) is shown as a compact header above it. The file list shows its ticket and status.

### Agent index and `mdv find`

Every new doc starts with an **agent index**: YAML frontmatter that tells an agent what the doc is about without reading all of it. The skill makes agents write one, and keep it current.

```yaml
---
ticket: TAP-4968
repo: web-monorepo
apps: [web-app]
type: investigation        # investigation | plan | summary | how-to | reference | bug
status: done               # in-progress | done | abandoned | superseded
summary: >
  web-app can drop Axios for Nuxt $fetch; api-sdk already uses HttpAdapter.
keywords: [axios, fetch, ofetch, http]
paths: [packages/http, packages/api-sdk]
pr: 1787
caveats:
  - Interceptors move to ofetch hooks
related: [TAP-4398]
updated: 2026-11-16
---
# 🔍 TAP-4968: Migrating web-app off Axios
```

Required: `repo`, `type`, `status`, `summary`, `keywords`, `updated`. The rest are optional. The full field list and rules are in the [skill](skills/mdv-docs/SKILL.md).

- **It stays local.** mdv strips it before Notion and gists.
- **Editing only the index doesn't count as a content change,** so updating status or the PR doesn't mark a published doc as needing a re-push.
- **In the viewer,** it shows as a header box.

`mdv find` searches the docs folder, best match first. It's read-only and builds its index on the fly; there's no index file to keep in sync.

```sh
mdv find axios fetch                       # words: ticket, keywords, paths, title, summary, caveats, text
mdv find --ticket TAP-4968                 # a ticket, and docs that list it under related
mdv find --repo web-monorepo --app web-app --status in-progress
mdv find --path packages/http              # docs whose paths overlap a folder or file
mdv find fetch --json                      # for agents: full index, score, and why it matched
mdv find --unindexed                       # docs without an agent index
```

Docs without an agent index still match: their repo and app come from the folder (`{repo}/{app}/…`), and the ticket from the file name (`TAP-1234-…`).

### Templates: `mdv new` and `mdv check`

Each doc type has a fixed set of sections, so a later reader (often an agent) can jump straight to what they need. The full guide, including how to write for the next agent, is [`templates.md`](skills/mdv-docs/templates.md).

| Type | Sections |
|---|---|
| `investigation` | Question · Findings · Options considered · Recommendation · Open questions · For future work |
| `plan` | Goal · Approach · Steps · Risks · Out of scope · For future work |
| `summary` | What changed · Why · How to verify · Caveats · Follow-ups · For future work |
| `bug` | Symptom · Root cause · Fix · How to verify · Related areas at risk · For future work |
| `how-to` | When to use · Steps · Troubleshooting |
| `reference` | Overview · Details · Gotchas |

```sh
mdv new summary TAP-333 fetch-header-bug   # skeleton with the agent index filled in
mdv check web-monorepo/web-app/TAP-333-fetch-header-bug.md
mdv check --all                            # every doc with an agent index
```

- **`mdv new`**
  - Run it inside the repo the work is about. It takes repo and app from the folder you're in (`apps/<app>` or `packages/<app>`), the branch from git, and the ticket and dates from the arguments.
  - `--repo`/`--app` override them, `--print` prints instead of writing, and it never overwrites an existing doc.
- **`mdv check`** is read-only and exits with 1 on errors.
  - **Errors:** a missing or invalid agent index, missing required fields, an unknown type or status, missing sections.
  - **Warnings:** empty sections in a done doc, relative links, wording like "yesterday" or "as discussed", a summary that only repeats the title, and an in-progress doc not updated for 30 days.
- `mdv notion push` and `mdv share` print a warning when a doc with an agent index has issues.

### Notion: `mdv notion`

```sh
mdv notion status                                  # every doc and its sync state (read-only)
mdv notion push doc.md --parent <page URL or ID>   # create a page under a parent page
mdv notion push doc.md --private                   # create a top-level private page
mdv notion push doc.md --database <database URL>   # add a new entry to an existing database
mdv notion push doc.md                             # update the page mdv created
mdv notion diff doc.md                             # what an update would change (read-only)
mdv notion schema <database URL>                   # a database's properties and options (read-only)
mdv notion info <page URL>                         # a page's title, parent, and whether mdv created it (read-only)
```

- The doc's `# H1` becomes the page title, and a leading emoji becomes the icon. New pages ask for a title, and `--title` sets it.
- Local images (`![caption](doc/screenshot.png)`) are uploaded and placed where they appear. Unchanged images are reused.
- Every push asks for confirmation (`--yes` skips the prompt). Updates show a diff first.
- mdv only touches pages listed in `<docs>/.notion.json`. It refuses to push over content edited in Notion since the last push.
- **Who can see the page:** a new page gets the access of where it's created. With `--private` only you can see it, and mdv can't change sharing (Notion's API has no way to). To make a doc visible to your team, push it under a shared page or into a shared database.

#### Notion-flavored Markdown

mdv sends docs as [Notion-flavored Markdown](https://developers.notion.com/guides/data-apis/notion-flavored-markdown.md) (`markdown_version: v2`): GFM plus XML-style tags for Notion blocks Markdown doesn't have.

- **Plain GFM is the default and always works.** Existing docs don't need changes.
- **Notion tags are optional:** toggles (drawers), callouts, columns, tabs, colored text and more. They look native in Notion. The mdv viewer and `mdv share` show a readable fallback, for example a toggle becomes a collapsible `<details>` in a gist.
- **Every element that differs from plain Markdown** is documented, with syntax and fallbacks, in [`skills/mdv-docs/notion-markdown.md`](skills/mdv-docs/notion-markdown.md).
- Notion's warnings (for example an unknown tag kept as text) are printed after a push.

```md
<toggle>
Full error log

The log goes here. It's hidden until someone opens the toggle.
</toggle>

<callout icon="⚠️" color="yellow_bg">
Run the migration before deploying.
</callout>
```

Pages pushed before v2 keep working. Their next push switches them over.

#### Databases

`--database` takes a database URL (a link to one of its views works too) or ID, and adds the doc as a new entry. `--prop "Name=value"` fills in the entry's properties. Repeat it for each property:

```sh
mdv notion push doc.md --database <database URL> \
  --prop "Category=Explanation" --prop "Audience=Developer,QA" --prop "Date=2026-11-16"
mdv notion push doc.md --prop "Category=Reference"   # later: change a property of the entry mdv created
```

| Property type | Value |
|---|---|
| select, status | one existing option |
| multi_select | existing options, comma-separated |
| date | `YYYY-MM-DD` or an RFC 3339 timestamp |
| text, number, checkbox, url, email, phone | the value (`true`/`false` for checkboxes) |
| relation (one-way only) | page URLs or IDs, comma-separated |

Option names match without case, and a leading emoji can be left out (`Tags=Technical Documentation` matches `⚙️ Technical Documentation`). The title comes from the H1 or `--title`, not `--prop`.

mdv keeps databases as they are. It only adds entries and edits the entries it created:

- It never creates databases or changes their properties.
- **It never adds options.** An unknown select value is refused instead of being added to the database.
- **It never sets two-way relations,** because linking would also change the linked pages. Set those by hand in Notion.

### Secret links: `mdv share`

```sh
mdv share doc.md         # create a secret gist and print the link
mdv share doc.md         # after edits: updates the same link
mdv share status         # shared docs: ✓ shared / ● changed / ✗ missing
mdv share rm doc.md      # delete the gist; the link stops working
```

- Local images are uploaded into the gist, and the shared copy links to them. Your file is never changed.
- The gist page shows the rendered doc. Viewers don't need a GitHub account.
- Shares are recorded in `<docs>/.share.json`. mdv only updates or deletes gists listed there.

> ⚠️ A secret gist is **unlisted, not access-controlled**. Anyone who gets the link can read it. Don't share credentials, customer data or security findings this way.

### Checking the setup: `mdv doctor`

```sh
mdv doctor
```

```
✓ config   ~/.config/mdv/.env
✓ docs     ~/dev/md (19 docs)
✓ notion   API token works (integration "mdv")
✓ git      found
✓ github   GITHUB_TOKEN works (your-login); it needs gist read/write access
```

## Configuration

mdv reads `~/.config/mdv/.env` (or `$XDG_CONFIG_HOME/mdv/.env`) on start. Real environment variables take priority over the file. All settings are optional.

- **`mdv init`** creates or updates the file interactively. Tokens are typed hidden and checked, and the previous file is backed up to `.env.bak`.
- **`mdv init --edit`** opens it in `$EDITOR`, creating it from the built-in template first.
- [`.env.example`](.env.example) is the same template, if you'd rather copy it by hand.

| Variable | Default | Effect |
|---|---|---|
| `MDV_DIR` | `~/dev/md` | The docs folder |
| `NOTION_API_TOKEN` | unset | Talk to Notion directly with this token, without `ntn` |
| `GITHUB_TOKEN` | unset | Create gists with this token, without `gh` |
| `MDV_ENV_FILE` | unset | Read settings from this file instead of `~/.config/mdv/.env` |

### Keeping the settings file somewhere else

To keep your `.env` in another place, for example in your mdv clone, point `MDV_ENV_FILE` at it from your shell profile:

```sh
echo 'export MDV_ENV_FILE=~/dev/mdv/.env' >> ~/.zshrc && source ~/.zshrc
```

- `MDV_ENV_FILE` has to be set in your shell. mdv can't read it from inside a settings file.
- If it's set, the file must exist, otherwise mdv stops with an error rather than silently running without your settings.
- In the mdv clone, `.gitignore` already excludes `.env`, so it won't be committed. But an agent working on mdv's code can still read it, unless your agent is set up to block access to `.env` files. Examples are a permission rule such as `Read(**/.env)` in Claude Code's deny list, or a plugin like FirstVet's [env-protection](https://github.com/firstvetcom/awesome-list/tree/main/src/ai/plugins/env-protection-opencode) for OpenCode.

### Notion access

There are two independent options. mdv picks one automatically: the token if `NOTION_API_TOKEN` is set, otherwise `ntn`.

**Option A: integration token (recommended for new setups).** No extra tools needed.

1. Go to https://www.notion.so/profile/integrations, then **New integration**. Pick the workspace and choose type **Internal**.
2. Under **Capabilities**, enable *Read content*, *Update content* and *Insert content*.
3. Copy the **Internal Integration Secret** into `~/.config/mdv/.env` as `NOTION_API_TOKEN=ntn_…`.
4. **Give the integration access to the pages you'll push under.** Open the parent page, then **•••** → **Connections** → add your integration. Child pages inherit the access.

> 💡 An integration only sees pages that have been explicitly connected to it. If `mdv notion push --parent …` says the page can't be found, connect the integration to that page. `--private` (a top-level page with no parent) may be refused for internal integrations, so use `--parent` or `--database` with a token. For `--database`, connect the integration to the database.

**Option B: the `ntn` CLI.** It uses your own Notion login, so it sees everything you can see.

1. Install Notion's `ntn` CLI and run `ntn login`.
2. Leave `NOTION_API_TOKEN` unset.

Both options send the same Notion API requests, so pages pushed with one can be updated with the other.

### GitHub access

There are two independent options. mdv uses the token if `GITHUB_TOKEN` is set, otherwise `gh`.

**Option A: personal access token.**

1. Go to https://github.com/settings/personal-access-tokens, then **Generate new token** (fine-grained).
2. Under **Account permissions**, set **Gists** to *Read and write*. No repository access is needed.
3. Put it in `~/.config/mdv/.env` as `GITHUB_TOKEN=github_pat_…`.

A classic token with only the `gist` scope works too.

**Option B: the `gh` CLI.**

```sh
brew install gh
gh auth login            # then, if needed:
gh auth refresh -s gist
```

Either way, gists are created on the account the token or `gh` belongs to. Images are pushed over HTTPS, so no SSH key setup is needed.

## Troubleshooting

| Problem | Fix |
|---|---|
| `mdv: command not found` | Put `~/.local/bin` on your `PATH` (setup step 2). |
| `no terminal to confirm on` | Run it in a terminal, or pass `--yes` once you're sure. |
| Notion: page not found / `object_not_found` | With a token: connect the integration to the page (**•••** → **Connections**). |
| Notion: `API token is invalid` | Check `NOTION_API_TOKEN` in `~/.config/mdv/.env`. |
| `… was edited in Notion since the last push` | Intentional. Copy the Notion edits into your local doc, or ask before overwriting. mdv won't work around it. |
| Share: `HTTP 401/403/404` from GitHub | The token is wrong or lacks *Gists: read and write*. |
| Share: `gh is logged in without the gist scope` | `gh auth refresh -s gist` |
| `⚠️ Notion: unsupported_tag …` | A tag Notion doesn't know was kept as text. Escape it as `\<tag>` or see [notion-markdown.md](skills/mdv-docs/notion-markdown.md). |
| `Markdown errors blocked parsing` | The doc isn't valid Notion-flavored Markdown. Nothing was saved. See the rules in [notion-markdown.md](skills/mdv-docs/notion-markdown.md#4-plain-markdown-rules-that-changed-in-v2). |
| Anything else | Run `mdv doctor`. |

## Development

```sh
make build            # ./mdv
go vet ./... && gofmt -l .
```

| File | Contents |
|---|---|
| `main.go` | CLI entry, docs folder, argument handling |
| `ui.go`, `style.go` | Terminal UI (Bubble Tea + Glamour) |
| `env.go` | `~/.config/mdv/.env` loading |
| `notion.go` | `mdv notion` commands, sync state, diffs |
| `notionapi.go` | Notion backends: direct API (token) or `ntn` |
| `images.go` | Local image detection, Notion uploads, link rewriting |
| `share.go` | `mdv share`: gists via GitHub API (token) or `gh` |
| `notionread.go` | `mdv notion schema` and `mdv notion info` (read-only) |
| `frontmatter.go` | Agent index (YAML frontmatter) parsing and stripping |
| `find.go` | `mdv find` |
| `doctypes.go` | Doc types, their sections, and `mdv new` |
| `check.go` | `mdv check` |
| `nfm.go` | Notion-flavored Markdown fallbacks for the viewer and gists |
| `skill.go`, `skills/mdv-docs/` | The bundled agent skill and `mdv skill` |
| `doctor.go` | `mdv doctor` |
| `init.go` | `mdv init` |
| `version.go` | `mdv --version`, docs folder + welcome doc on first run |
| `contrib/claude-code/mdv-guard.sh` | Optional Claude Code hook that forces approval for agent writes |
| `install.sh` | The install script |
| `.goreleaser.yaml`, `.github/workflows/release.yml` | Release pipeline |

### Releasing

Push a version tag and GitHub Actions does the rest. GoReleaser builds macOS and Linux binaries and publishes them as a GitHub Release, which `install.sh` and the README links pick up.

```sh
git tag v0.1.0 && git push origin v0.1.0
```

To try the pipeline locally without publishing anything: `go run github.com/goreleaser/goreleaser/v2@v2.12.7 release --snapshot --clean` (output in `dist/`).
