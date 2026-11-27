# Welcome to mdv

A terminal viewer for the Markdown files in `~/dev/md`.

## Getting around

- **Scroll** with the mouse wheel, `↑/↓` or `j/k`
- **Page** with `space` / `b` or `PgUp` / `PgDn`
- **Jump** to the top or bottom with `g` / `G`
- **Reload** the file from disk with `r`
- **Back** to the file list with `esc`

## Opening other files

```sh
mdv                         # browse ~/dev/md
mdv README.md               # relative path
mdv ~/dev/some-repo/docs.md # absolute or ~ path
```

## Finding docs

Docs written by an agent start with an **agent index**: a short YAML block with the ticket, repo, summary and caveats. It's shown as a header above the doc, and it stays local.

```sh
mdv find axios fetch          # best match first
mdv find --ticket TAP-1234    # one ticket, and docs related to it
mdv new summary TAP-1234 x    # start a doc from a template
mdv check <file.md>           # check it
```

## Rendering check

| Feature     | Supported |
|-------------|-----------|
| Headings    | yes       |
| Tables      | yes       |
| Code blocks | yes       |

> Drop any `.md` file into `~/dev/md` (subfolders work too) and it shows up in the list.

### Filler so you can test scrolling

1. Lorem ipsum dolor sit amet, consectetur adipiscing elit.
2. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
3. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.
4. Duis aute irure dolor in reprehenderit in voluptate velit esse.
5. Excepteur sint occaecat cupidatat non proident, sunt in culpa.
6. Curabitur pretium tincidunt lacus, nulla gravida orci a odio.
7. Nullam varius, turpis et commodo pharetra, est eros bibendum elit.
8. Integer in mauris eu nibh euismod gravida.
9. Duis ac tellus et risus vulputate vehicula.
10. Donec lobortis risus a elit, etiam tempor.

That's it! Press `esc` to go back or `q` to quit.
