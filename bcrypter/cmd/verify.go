/*
Copyright © 2025 NAME HERE w00j00ng351@gmail.com
*/
package cmd

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var bcryptHashPattern = regexp.MustCompile(`^\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}$`)

func newVerifyCommand(readPassword func() ([]byte, error)) *cobra.Command {
	var hash, password string
	command := &cobra.Command{
		Use:   "verify",
		Short: "Verify a password against a bcrypt hash",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("hash") {
				fmt.Fprint(cmd.OutOrStdout(), "Enter Hash: ")
				value, err := readPassword()
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("read hash: %w", err)
				}
				hash = string(value)
			}
			if !cmd.Flags().Changed("password") {
				fmt.Fprint(cmd.OutOrStdout(), "Enter Password: ")
				value, err := readPassword()
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(value)
			}
			if !bcryptHashPattern.MatchString(hash) {
				return errors.New("invalid bcrypt hash")
			}
			if len(password) > 72 {
				return bcrypt.ErrPasswordTooLong
			}
			err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
			if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				fmt.Fprintln(cmd.OutOrStdout(), "Password does not match.")
				return err
			}
			if err != nil {
				return fmt.Errorf("invalid bcrypt hash: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Password is valid.")
			return nil
		},
	}
	command.Flags().StringVar(&hash, "hash", "", "bcrypt hash to verify against")
	command.Flags().StringVarP(&password, "password", "p", "", "password to verify")
	return command
}
