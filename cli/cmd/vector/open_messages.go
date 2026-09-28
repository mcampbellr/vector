package main

import "strings"

// openMessages are the user-facing report lines of `vector open`, localized by
// config.language. Ids, paths, branches and session/window names are never
// translated; errors stay in English like the rest of the binary.
type openMessages struct {
	worktreeReused     string // %s rel path
	worktreeCreated    string // %s rel path
	worktreeNone       string
	createPrompt       string // %s rel path, %s source (one of the source* lines)
	sourceNewBranch    string // %s branch, %s base
	sourceLocalBranch  string // %s branch
	sourceRemoteBranch string // %s branch, %s remote branch
	activeSpec         string // %s display path
	activeSpecFailed   string // %v error
	statusNext         string // %s status, %s slash command
	statusNoClaude     string // %s status, %s slash command
	statusOnly         string // %s status
	noNextCommand      string // %s status
	windowCreated      string // %s window name, %s session
	windowFocused      string // %s target
	paneSplit          string
	pickHeader         string
}

var openMessagesEN = openMessages{
	worktreeReused:     "worktree %s (reused)",
	worktreeCreated:    "worktree %s (created)",
	worktreeNone:       "no worktree, using the repo root",
	createPrompt:       "Create worktree %s (%s)? [y/N] ",
	sourceNewBranch:    "new branch %s from %s",
	sourceLocalBranch:  "existing branch %s",
	sourceRemoteBranch: "branch %s tracking %s",
	activeSpec:         "active spec → %s",
	activeSpecFailed:   "active spec not recorded: %v",
	statusNext:         "status %s · next %s",
	statusNoClaude:     "status %s · next %s (not launched: --no-claude)",
	statusOnly:         "status %s",
	noNextCommand:      "no next command (%s)",
	windowCreated:      "tmux window %s in session %s",
	windowFocused:      "existing window focused (%s)",
	paneSplit:          "tmux pane split in the current window",
	pickHeader:         "In-progress and focused specs:",
}

var openMessagesES = openMessages{
	worktreeReused:     "worktree %s (reusado)",
	worktreeCreated:    "worktree %s (creado)",
	worktreeNone:       "sin worktree, uso raíz del repo",
	createPrompt:       "Crear worktree %s (%s)? [y/N] ",
	sourceNewBranch:    "rama nueva %s desde %s",
	sourceLocalBranch:  "rama existente %s",
	sourceRemoteBranch: "rama %s siguiendo %s",
	activeSpec:         "spec activo → %s",
	activeSpecFailed:   "spec activo sin registrar: %v",
	statusNext:         "estado %s · próximo %s",
	statusNoClaude:     "estado %s · próximo %s (sin lanzar: --no-claude)",
	statusOnly:         "estado %s",
	noNextCommand:      "sin próximo comando (%s)",
	windowCreated:      "tmux ventana %s en sesión %s",
	windowFocused:      "ventana existente enfocada (%s)",
	paneSplit:          "tmux panel dividido en la ventana actual",
	pickHeader:         "Specs en progreso y en foco:",
}

// openMessagesFor picks the report language from a BCP-47 tag: Spanish for "es"
// and its regional variants, English otherwise (the binary's default).
func openMessagesFor(language string) openMessages {
	lang := strings.ToLower(strings.TrimSpace(language))
	if lang == "es" || strings.HasPrefix(lang, "es-") || strings.HasPrefix(lang, "es_") {
		return openMessagesES
	}
	return openMessagesEN
}
