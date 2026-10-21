/*
Copyright © 2023 w00j00ng351@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func newRootCommand(readPassword func() ([]byte, error), copyHash func([]byte) error) *cobra.Command {
	var onClipboard bool
	command := &cobra.Command{
		Use:           "bcrypter",
		Short:         "A simple CLI tool to hash passwords using bcrypt",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.ErrOrStderr(), "Enter Password: ")
			plain, err := readPassword()
			fmt.Fprintln(cmd.ErrOrStderr())
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			hash, err := bcrypt.GenerateFromPassword(plain, bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if onClipboard {
				if err := copyHash(hash); err != nil {
					return fmt.Errorf("copy hash: %w", err)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "copied on the clipboard!")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), string(hash))
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
	}, writeClipboard)
	if err := command.Execute(); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
