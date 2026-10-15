package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/scrypter/password"
	"golang.design/x/clipboard"
	"golang.org/x/term"
)

var errPasswordMismatch = errors.New("password does not match")

func newRootCommand(readPassword func() ([]byte, error)) *cobra.Command {
	var onClipboard bool
	command := &cobra.Command{
		Use:           "scrypter",
		Short:         "Hash passwords using Better Auth's default scrypt format",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), "Enter Password: ")
			plain, err := readPassword()
			fmt.Fprintln(cmd.OutOrStdout())
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			hash, err := password.Hash(string(plain))
			if err != nil {
				return err
			}
			if onClipboard {
				if err := clipboard.Init(); err != nil {
					return fmt.Errorf("initialize clipboard: %w", err)
				}
				clipboard.Write(clipboard.FmtText, []byte(hash))
				fmt.Fprintln(cmd.OutOrStdout(), "copied on the clipboard!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), hash)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&onClipboard, "clipboard", false, "copy hash to clipboard")
	command.AddCommand(newVerifyCommand(readPassword))
	return command
}

// Execute runs the CLI, using exit codes 0 for success, 1 for mismatch, and 2 for errors.
func Execute() {
	command := newRootCommand(func() ([]byte, error) {
		return term.ReadPassword(int(syscall.Stdin))
	})
	if err := command.Execute(); err != nil {
		if errors.Is(err, errPasswordMismatch) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
