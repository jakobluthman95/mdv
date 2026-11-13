# mdv

A documentation workspace for AI coding agents. Your agent writes its findings, plans and summaries to one local folder, you read and review them in the terminal, and the agent publishes them to Notion or a secret link, but only once you approve.

## What and why

Coding agents produce a lot of useful writing: investigation notes, plans, findings, summaries of the work done. Without a convention, it ends up scattered in repos, chat scrollback or `LOCAL_DOCS/` folders that never get shared.

mdv gives agents (and you) one place and one workflow for it:

1. **Write:** the [mdv-docs skill](#ai-agents) tells the agent where docs go (one folder outside your repos, default `~/dev/md`, organised by repo) and how to format them so they publish cleanly.
2. **Review:** `mdv` opens a terminal viewer over every doc, newest first, so you can read what the agent wrote without leaving the terminal.
3. **Publish, with approval:** the agent can push a doc to Notion (`mdv notion`) or turn it into a secret link (`mdv share`). The skill makes it ask first and show you exactly what will change.

The commands are built so an agent can use them safely:

- **Read-only checks first:** `status`, `diff` and `doctor` never write anything.
- **Confirmation by default:** every write asks for confirmation. `--yes` is only passed after you've approved.
- **mdv only touches what it created:** it updates only the Notion pages and gists it made itself, recorded in `.notion.json` and `.share.json`.
- **No silent overwrites:** it refuses to overwrite edits someone made in Notion.
- **Local files stay the source of truth:** mdv never edits your docs.

You can use all of it by hand too. The agent is just the main user.

## Requirements

**Prerequisites** (install them however you normally do):

- Go 1.27+, to build mdv
- `make`, to build and install
- `git`, to clone the repo; `mdv share` also uses it to upload images

**Set up in the steps below:**

| What | Needed for | Where |
|---|---|---|
| `~/.local/bin` on your `PATH` | Running `mdv` | See step 2 below |
| A Notion **integration token** *or* the `ntn` CLI | `mdv notion` only | See [Notion access](#notion-access) |
| A GitHub **token** *or* the `gh` CLI | `mdv share` only | See [GitHub access](#github-access) |

Browsing docs needs only the binary.

## Setup, step by step

**1. Get the code and build it.**

```sh
git clone git@github.com:jakobluthman95/mdv.git ~/dev/mdv
cd ~/dev/mdv
make install        # builds ~/.local/bin/mdv and creates ~/dev/md with a welcome doc
```

**2. Put `~/.local/bin` on your `PATH`** if it isn't there yet:

```sh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

**3. Create your settings file** (optional, but needed for tokens or a different docs folder):

```sh
mkdir -p ~/.config/mdv
cp .env.example ~/.config/mdv/.env
chmod 600 ~/.config/mdv/.env    # it holds secrets
```

Then fill in what you need (see [Configuration](#configuration)).

**4. Give your agent the skill.** This is the step that makes mdv useful to an agent:

```sh
pnpx skills add git@github.com:jakobluthman95/mdv.git --skill mdv-docs --global
```

See [AI agents](#ai-agents) for other ways to install it.

**5. Set up Notion and/or GitHub access** if you want the agent to publish with `mdv notion` or `mdv share`. See [Configuration](#configuration).

**6. Check everything:**

```sh
mdv doctor
```

`doctor` is read-only. It checks the config file, the docs folder, Notion access and GitHub access, and tells you what's missing.

### Updating

```sh
cd ~/dev/mdv && git pull && make install
pnpx skills update     # picks up skill changes too
```

## AI agents

The skill is what turns mdv from a viewer into an agent workflow. [`skills/mdv-docs/SKILL.md`](skills/mdv-docs/SKILL.md) tells the agent:

- where docs go and how to name them;
- the Notion-ready Markdown format (H1 title, GFM only, images next to the doc);
- how to push to Notion and share by link, using the read-only commands first;
- to **always ask before any write**, stating exactly what will happen.

It uses the open `SKILL.md` format and reads fine as plain Markdown.

Once it's installed, just talk to your agent: "write up these findings", "push that doc to Notion under the team page", "give me a link I can send to a colleague".

### Install with the skills CLI (recommended)

```sh
pnpx skills add git@github.com:jakobluthman95/mdv.git --skill mdv-docs --global
```

This uses the open [`skills`](https://github.com/vercel-labs/skills) installer (`npx skills …` works too):

- It clones the repo with your existing git login.
- It finds `skills/mdv-docs/SKILL.md` and installs it for every coding agent it detects (Claude Code, Codex, Cursor, OpenCode and more).
- `--global` installs it for your user, so it works in all projects. Leave it out to install into the current project only.

To keep it up to date:

```sh
pnpx skills check     # any updates?
pnpx skills update    # install them
```

Run `pnpx skills add git@github.com:jakobluthman95/mdv.git --list` to see what's in the repo without installing anything.

### Install by hand

- **Agents with skill folders:** copy `skills/mdv-docs` into the agent's skills directory, e.g. `mkdir -p ~/.claude/skills && cp -r skills/mdv-docs ~/.claude/skills/` for Claude Code. Re-copy it after `git pull` if it changed.
- **Anything else:** reference it from your global `AGENTS.md`, or paste it into your agent's instructions.

> 💡 If you already have a skill named `mdv-docs` in an agent's skills folder, remove it or skip that agent first. Otherwise the two will clash.

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

### Notion: `mdv notion`

```sh
mdv notion status                                  # every doc and its sync state (read-only)
mdv notion push doc.md --parent <page URL or ID>   # create a page under a parent page
mdv notion push doc.md --private                   # create a top-level private page
mdv notion push doc.md                             # update the page mdv created
mdv notion diff doc.md                             # what an update would change (read-only)
```

- The doc's `# H1` becomes the page title, and a leading emoji becomes the icon. New pages ask for a title, and `--title` sets it.
- Local images (`![caption](doc/screenshot.png)`) are uploaded and placed where they appear. Unchanged images are reused.
- Every push asks for confirmation (`--yes` skips the prompt). Updates show a diff first.
- mdv only touches pages listed in `<docs>/.notion.json`. It refuses to push over content edited in Notion since the last push.

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

mdv reads `~/.config/mdv/.env` (or `$XDG_CONFIG_HOME/mdv/.env`) on start. Real environment variables take priority over the file. All settings are optional. [`.env.example`](.env.example) is the template.

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

> 💡 An integration only sees pages that have been explicitly connected to it. If `mdv notion push --parent …` says the page can't be found, connect the integration to that page. `--private` (a top-level page with no parent) may be refused for internal integrations, so use `--parent` with a token.

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
| `doctor.go` | `mdv doctor` |
