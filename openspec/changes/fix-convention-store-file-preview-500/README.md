# fix-convention-store-file-preview-500

Bug fix: the board's file preview returns HTTP 500 ("could not read artifact") for any spec whose `SpecDoc` lives outside `.vector/` (convention store). `verifyArtifactPath`'s defense-in-depth allowlist omits the `SpecDoc`'s own location; the fix adds it (already-trusted committed state, traversal defense preserved) and closes the matching test gap.
