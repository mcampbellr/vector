# Root anchoring guardrail

A repo has exactly **one** Vector store. If a `.vector/` directory exists at any ancestor of
where you are working, that ancestor is the base — anchor to it and nothing else. Creating a
second `.vector/` in a subdirectory fragments the board across stores: the cards you write
become invisible on the board the user is looking at, and no command reads them back.

Concretely:

- **Never run `vector init` in a subdirectory of a repo that already has a store above it.**
  The binary refuses this by design and names the ancestor in the error. `--force` exists for
  the deliberate multi-store case only — never pass it to silence the error. If you believe
  the nested store is genuinely wanted, ask the user first.
- **Do not create `.vector/` by hand, ever.** The binary owns every write under it
  (CLI-owns-writes); a directory you create yourself is a stray no command anchors to.
- **A pre-existing stray is a diagnosis, not a workaround.** `vector doctor` lists stray
  stores; `vector doctor adopt <path> --force` migrates one into the canonical store and
  deletes it. That deletion is destructive — surface the plan (`adopt` without `--force`
  prints it and touches nothing) and get explicit user confirmation before passing `--force`.
- **When a command warns about a stray**, relay the warning to the user with the path it
  named. Silently working around it hides the fragmentation that caused it.

Subprojects inside a monorepo are expressed with the card's `repo` field inside the single
root store — not with a store per subproject.
