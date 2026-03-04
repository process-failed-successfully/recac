package main

import (
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(recac completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ recac completion bash > /etc/bash_completion.d/recac
  # macOS:
  $ recac completion bash > $(brew --prefix)/etc/bash_completion.d/recac

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ recac completion zsh > "${fpath[1]}/_recac"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ recac completion fish | source

  # To load completions for each session, execute once:
  $ recac completion fish > ~/.config/fish/completions/recac.fish

PowerShell:

  PS> recac completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> recac completion powershell > recac.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
