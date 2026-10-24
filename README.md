# mdv

A terminal Markdown viewer built with Bubble Tea and Glamour.

```sh
make install   # builds ~/.local/bin/mdv and seeds ~/dev/md/welcome.md
mdv            # browse .md files in ~/dev/md (recursive, newest first)
mdv path.md    # open any .md file (relative, absolute or ~ path)
mdv --help
```

Set `MDV_DIR` to use a different docs folder. Run `mdv --help` for the key bindings.
