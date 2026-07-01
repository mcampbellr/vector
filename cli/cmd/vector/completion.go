package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd implements `vector completion <bash|zsh|fish|powershell>`,
// generating the script on the fly from the command tree (nothing embedded —
// architecture/distribution-packaging.md imposes no embed requirement for
// completions). The script is written to stdout so it can be redirected to the
// shell's completion directory.
func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "generate a shell completion script",
		Long: `Generate an autocompletion script for your shell.

  # Bash
  vector completion bash > /usr/local/etc/bash_completion.d/vector

  # Zsh
  vector completion zsh > "${fpath[1]}/_vector"

  # Fish
  vector completion fish > ~/.config/fish/completions/vector.fish

  # PowerShell
  vector completion powershell > vector.ps1
`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("missing shell name. Usage: vector completion <bash|zsh|fish|powershell>")
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Generate against the root of the tree so completions cover every command.
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unknown shell %q: use bash|zsh|fish|powershell", args[0])
			}
		},
	}
}
