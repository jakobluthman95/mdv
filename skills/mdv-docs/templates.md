# 📐 Doc templates

Every doc has the same shape: an **agent index** (YAML frontmatter), one `# H1`, then the `##` sections for its type, in this order. `mdv new <type> [TICKET] <slug>` writes this skeleton for you, and `mdv check <file>` tells you what's missing.

Fixed sections let an agent jump straight to "Caveats" or "For future work" instead of reading everything.

## Writing for the next agent

A doc is read later, by someone, often an agent, who wasn't in this conversation. Write so they can act on it.

- **Name things.** Give file paths, components, functions, endpoints, commands, versions, and PR or commit links. Not "the HTTP layer", but `packages/http/src/client.ts`.
- **Give the reason for each decision.** "Chose X over Y because Z". Otherwise the next person undoes it.
- **Make the state explicit:** done, not done, open. Write "None." for an empty section in a finished doc, so it's clear nothing was forgotten.
- **No wording that only works in the moment.** Not "yesterday", "as discussed" or "the bug above". Use dates, links and names.
- **Put what matters for later work in "For future work":** what still applies, what's temporary, what breaks if someone changes it, and where to look first.
- **Keep the agent index current.** Update `status`, `pr` and `updated` as work moves on. When a newer doc replaces this one, set `status: superseded` and `superseded_by`.
- Add sections when you need them, but keep the required ones and their order.

## 🔍 `investigation`

For when you looked into something and want to record what you found.

| Section | What goes in it |
|---|---|
| **Question** | What was investigated and why. One or two sentences. |
| **Findings** | What's true, with evidence: paths, code references, measurements, links. |
| **Options considered** | Each option with its pros, cons and effort. |
| **Recommendation** | The option to take, and why. |
| **Open questions** | What's still unknown, and who could answer it. |
| **For future work** | What someone acting on this must know. |

## 📋 `plan`

For work that hasn't started yet.

| Section | What goes in it |
|---|---|
| **Goal** | The outcome, and how you'll know it's reached. |
| **Approach** | The chosen design, and why. |
| **Steps** | Ordered steps, each naming the files or areas it touches. Use a checklist. |
| **Risks** | What could go wrong, and the mitigation. |
| **Out of scope** | What this deliberately doesn't do. |
| **For future work** | What follows after this plan. |

## ✅ `summary`

For work that was done.

| Section | What goes in it |
|---|---|
| **What changed** | The changes, with paths and the PR. |
| **Why** | The reason, with links to the ticket or investigation. |
| **How to verify** | Commands or steps that show it works. |
| **Caveats** | Known limits, workarounds and surprises. |
| **Follow-ups** | What's left to do, ideally as tickets. |
| **For future work** | What someone changing this area later must know. |

## 🐛 `bug`

| Section | What goes in it |
|---|---|
| **Symptom** | What users or developers saw, where, and since when. |
| **Root cause** | Why it happened, with the code reference. |
| **Fix** | What was changed, with the PR. Or the proposed fix. |
| **How to verify** | How to reproduce the bug, and how to confirm the fix. |
| **Related areas at risk** | Other places with the same pattern. |
| **For future work** | How to avoid it next time. |

## 🛠️ `how-to`

| Section | What goes in it |
|---|---|
| **When to use** | The situation this solves. |
| **Steps** | Copy-pasteable commands and steps, in order. |
| **Troubleshooting** | Common errors and their fixes. |

## 📖 `reference`

| Section | What goes in it |
|---|---|
| **Overview** | What this covers, in a few sentences. |
| **Details** | The reference content itself: tables and lists. |
| **Gotchas** | Surprising behaviour and pitfalls. |
