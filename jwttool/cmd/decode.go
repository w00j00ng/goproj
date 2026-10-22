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
	return newReadCommand(false)
}

func newVerifyCommand() *cobra.Command {
	return newReadCommand(true)
}

func newReadCommand(verify bool) *cobra.Command {
	var algorithm string
	var key keyInput
	var policy token.Policy
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
			if verify {
				if command.Flags().Changed("require") && len(policy.Required) == 0 {
					return errors.New("required fields must not be empty")
				}
				for _, name := range []string{"iss", "sub", "aud", "jti", "typ"} {
					value, _ := command.Flags().GetString(name)
					if command.Flags().Changed(name) && value == "" {
						return errors.New("expected claim values must not be empty")
					}
				}
				for _, name := range []string{"max-age", "max-lifetime"} {
					value, _ := command.Flags().GetDuration(name)
					if command.Flags().Changed(name) && value <= 0 {
						return errors.New("maximum age and lifetime must be positive durations")
					}
				}
			}
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
			var result *token.Result
			var validationErr error
			if verify {
				result, validationErr = token.Verify(string(data), algorithm, verificationKey, policy, time.Now())
			} else {
				result, validationErr = token.Decode(string(data), algorithm, verificationKey, time.Now())
			}
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
	if verify {
		command.Use = "verify [token]"
		command.Short = "Verify a JWT signature, key length, and selected claim policies"
		command.Flags().Lookup("alg").Usage = "required algorithm: " + strings.Join(token.SupportedAlgorithms(), ", ")
		command.Flags().StringVar(&policy.Issuer, "iss", "", "required issuer value")
		command.Flags().StringVar(&policy.Subject, "sub", "", "required subject value")
		command.Flags().StringVar(&policy.Audience, "aud", "", "required audience (string or array membership)")
		command.Flags().StringVar(&policy.JWTID, "jti", "", "required JWT ID value (does not check replay or revocation)")
		command.Flags().StringVar(&policy.Type, "typ", "", "required header media type, e.g. JWT or at+jwt")
		command.Flags().StringSliceVar(&policy.Required, "require", nil, "required fields: iss,sub,aud,exp,nbf,iat,jti,typ (repeatable)")
		command.Flags().DurationVar(&policy.Leeway, "leeway", 0, "clock tolerance for exp, nbf, and iat")
		command.Flags().DurationVar(&policy.MaxAge, "max-age", 0, "maximum time since iat (requires iat; disabled if omitted)")
		command.Flags().DurationVar(&policy.MaxLifetime, "max-lifetime", 0, "maximum positive exp-iat interval (requires both; disabled if omitted)")
	}
	key.addFlags(command)
	return command
}
