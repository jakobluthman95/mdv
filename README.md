# mdv

A terminal Markdown viewer built with Bubble Tea and Glamour.

```sh
make install   # builds ~/.local/bin/mdv and seeds ~/dev/md/welcome.md
mdv            # browse .md files in ~/dev/md (recursive, newest first)
mdv path.md    # open any .md file (relative, absolute or ~ path)
mdv --help
```

## Sharing without Notion

```sh
mdv share docs/foo.md      # secret GitHub gist: only people with the link can read it
mdv share docs/foo.md      # again after edits: updates the same link
mdv share status           # shared docs: ✓ shared / ● changed
mdv share rm docs/foo.md   # delete the gist (link stops working)
```

Uses the `gh` CLI's logged-in account. Local images are uploaded into the gist and
linked by URL in the shared copy; the local doc is never changed. Shares are recorded
in `<docs>/.share.json`.

Set `MDV_DIR` to use a different docs folder. Run `mdv --help` for the key bindings.
