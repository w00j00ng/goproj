package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/jwttool/token"
)

func newEncodeCommand() *cobra.Command {
	var algorithm string
	var key keyInput
	command := &cobra.Command{
		Use:   "encode",
		Short: "Sign a JSON claims object using an enabled JWT algorithm",
		Args:  noArguments,
		RunE: func(command *cobra.Command, args []string) error {
			body, err := readInput(command, "body")
			if err != nil {
				return err
			}
			if algorithm == "" {
				return errors.New("--alg is required")
			}
			signingKey, err := key.read(command)
			if err != nil {
				return err
			}
			raw, err := token.Encode(body, algorithm, signingKey)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(command.OutOrStdout(), raw)
			return err
		},
	}
	command.Flags().String("body", "", "JSON claims object")
	command.Flags().String("body-file", "", "JSON claims file, or - for stdin")
	command.Flags().StringVar(&algorithm, "alg", "", "required signing algorithm: "+strings.Join(token.SupportedAlgorithms(), ", "))
	key.addFlags(command)
	return command
}
