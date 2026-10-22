package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/jwttool/token"
)

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:           "jwttool",
		Short:         "Inspect, verify, and sign JWTs using " + strings.Join(token.SupportedAlgorithms(), ", "),
		Args:          noArguments,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, args []string) error {
			return command.Help()
		},
	}
	command.SetFlagErrorFunc(func(_ *cobra.Command, _ error) error {
		return errors.New("invalid flags: check names and values with --help")
	})
	command.CompletionOptions.DisableDefaultCmd = true
	command.AddCommand(newDecodeCommand(), newEncodeCommand(), newVerifyCommand())
	command.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(_ *cobra.Command, args []string) error {
			target, remaining, err := command.Find(args)
			if err != nil || len(remaining) != 0 {
				return errors.New("unknown help topic: use help decode, help encode, or help verify")
			}
			return target.Help()
		},
	})
	return command
}

func noArguments(_ *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errors.New("unexpected positional arguments: use flags shown in --help")
	}
	return nil
}

// Execute returns 0 for success, 1 for validation failure, or 2 for input/runtime errors.
func Execute() int {
	if err := newRootCommand().Execute(); err != nil {
		if errors.Is(err, token.ErrValidation) {
			return 1
		}
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	return 0
}

func readInput(command *cobra.Command, name string) ([]byte, error) {
	if command.Flags().Changed(name) == command.Flags().Changed(name+"-file") {
		return nil, fmt.Errorf("provide exactly one of --%s or --%s-file", name, name)
	}
	if command.Flags().Changed(name) {
		value, _ := command.Flags().GetString(name)
		if len(value) > token.MaxInputSize {
			return nil, fmt.Errorf("%s exceeds 1 MiB limit", name)
		}
		return []byte(value), nil
	}
	path, _ := command.Flags().GetString(name + "-file")
	if path == "-" {
		return readLimited(command.InOrStdin(), token.MaxInputSize, name)
	}
	return readFile(path, token.MaxInputSize, name)
}

func readLimited(reader io.Reader, limit int, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: input failed", name)
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s exceeds %d byte limit", name, limit)
	}
	return data, nil
}

func readFile(path string, limit int, name string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s file: cannot open file", name)
	}
	defer file.Close()
	return readLimited(file, limit, name)
}

type keyInput struct {
	secret    string
	secretEnv string
	keyFile   string
}

func (input *keyInput) addFlags(command *cobra.Command) {
	command.Flags().StringVar(&input.secret, "secret", "", "HMAC secret (prefer --secret-env to avoid shell history)")
	command.Flags().StringVar(&input.secretEnv, "secret-env", "", "environment variable containing the HMAC secret")
	command.Flags().StringVar(&input.keyFile, "key-file", "", "PEM key file (private for encode, public for decode/verify)")
}

func (input keyInput) read(command *cobra.Command) (token.Key, error) {
	count := 0
	for _, name := range []string{"secret", "secret-env", "key-file"} {
		if command.Flags().Changed(name) {
			count++
		}
	}
	if count > 1 {
		return token.Key{}, errors.New("choose only one key input: --secret, --secret-env, or --key-file")
	}
	if command.Flags().Changed("secret") {
		if input.secret == "" {
			return token.Key{}, errors.New("secret must not be empty")
		}
		return token.Key{Secret: []byte(input.secret)}, nil
	}
	if command.Flags().Changed("secret-env") {
		secret := os.Getenv(input.secretEnv)
		if secret == "" {
			return token.Key{}, errors.New("secret environment variable must be set and nonempty")
		}
		return token.Key{Secret: []byte(secret)}, nil
	}
	if command.Flags().Changed("key-file") {
		data, err := readFile(input.keyFile, token.MaxKeySize, "key")
		if err != nil {
			return token.Key{}, err
		}
		return token.Key{PEM: data}, nil
	}
	return token.Key{}, nil
}
