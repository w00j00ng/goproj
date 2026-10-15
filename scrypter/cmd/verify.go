package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/scrypter/password"
)

func newVerifyCommand(readPassword func() ([]byte, error)) *cobra.Command {
	var stored, plain string
	command := &cobra.Command{
		Use:   "verify",
		Short: "Verify a password against a Better Auth scrypt hash",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("hash") {
				fmt.Fprint(cmd.OutOrStdout(), "Enter Hash: ")
				value, err := readPassword()
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("read hash: %w", err)
				}
				stored = string(value)
			}
			if !cmd.Flags().Changed("password") {
				fmt.Fprint(cmd.OutOrStdout(), "Enter Password: ")
				value, err := readPassword()
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				plain = string(value)
			}
			match, err := password.Verify(stored, plain)
			if err != nil {
				return err
			}
			if !match {
				fmt.Fprintln(cmd.OutOrStdout(), "Password does not match.")
				return errPasswordMismatch
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Password is valid.")
			return nil
		},
	}
	command.Flags().StringVar(&stored, "hash", "", "Better Auth scrypt hash to verify against")
	command.Flags().StringVarP(&plain, "password", "p", "", "password to verify")
	return command
}
