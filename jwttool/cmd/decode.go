package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/jwttool/token"
)

func newDecodeCommand() *cobra.Command {
	var algorithm string
	var key keyInput
	command := &cobra.Command{
		Use:   "decode [token]",
		Short: "Inspect a JWT, optionally verifying its signature and exp/nbf claims",
		Args: func(command *cobra.Command, args []string) error {
			inputs := len(args)
			for _, name := range []string{"token", "token-file"} {
				if command.Flags().Changed(name) {
					inputs++
				}
			}
			if inputs != 1 {
				return errors.New("provide exactly one token input: positional token, --token, or --token-file")
			}
			return nil
		},
		RunE: func(command *cobra.Command, args []string) error {
			var data []byte
			var err error
			if len(args) == 1 {
				data = []byte(args[0])
			} else {
				data, err = readInput(command, "token")
			}
			if err != nil {
				return err
			}
			verificationKey, err := key.read(command)
			if err != nil {
				return err
			}
			result, validationErr := token.Decode(string(data), algorithm, verificationKey, time.Now())
			if result == nil {
				return validationErr
			}
			encoder := json.NewEncoder(command.OutOrStdout())
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(result); err != nil {
				return err
			}
			return validationErr
		},
	}
	command.Flags().String("token", "", "compact JWT to inspect")
	command.Flags().String("token-file", "", "JWT file, or - for stdin")
	command.Flags().StringVar(&algorithm, "alg", "", "expected algorithm: "+strings.Join(token.SupportedAlgorithms(), ", ")+" (default: read header)")
	key.addFlags(command)
	return command
}
