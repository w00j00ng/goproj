package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"time"
	_ "time/tzdata"

	"github.com/spf13/cobra"
)

const (
	minUnix = -62167219200
	maxUnix = 253402300799
)

var rfc3339Syntax = regexp.MustCompile(
	"^[0-9]{4}-[0-9]{2}-[0-9]{2}T(?:[01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9]" +
		"(?:\\.[0-9]{1,9})?(?:Z|[+-](?:[01][0-9]|2[0-3]):[0-5][0-9])$")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	root := &cobra.Command{
		Use:           "timetool",
		Short:         "Convert current or supplied times between Unix seconds, milliseconds, and RFC3339",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return writeTime(stdout, now, "rfc3339", "UTC")
		},
	}
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.CompletionOptions.DisableDefaultCmd = true

	// Cobra's help callback cannot return a write error.
	var helpErr error
	root.SetHelpFunc(func(command *cobra.Command, _ []string) {
		_, helpErr = fmt.Fprint(command.OutOrStdout(), command.Short+"\n\n"+command.UsageString())
	})
	root.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(_ *cobra.Command, args []string) error {
			target, remaining, err := root.Find(args)
			if err != nil || len(remaining) != 0 {
				return errors.New("unknown help topic: use help unix, help ms, or help rfc3339")
			}
			return target.Help()
		},
	})

	for _, output := range []string{"unix", "ms", "rfc3339"} {
		var from, zone string
		command := &cobra.Command{
			Use:   output + " [value]",
			Short: "Convert a time to " + output + " (current time when value is omitted)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(command *cobra.Command, values []string) error {
				t := now
				if len(values) == 0 && command.Flags().Changed("from") {
					return errors.New("value is required when --from is set")
				}
				if len(values) != 0 {
					var err error
					t, err = readTime(values[0], from)
					if err != nil {
						return err
					}
				}
				return writeTime(stdout, t, output, zone)
			},
		}
		command.Flags().StringVar(&from, "from", "rfc3339", "input format: rfc3339, unix (seconds), or ms (milliseconds)")
		if output == "rfc3339" {
			command.Flags().StringVar(&zone, "tz", "UTC", "output time zone: UTC, an IANA name, or Local")
		}
		root.AddCommand(command)
	}
	err := root.Execute()
	if err == nil {
		err = helpErr
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func readTime(value, from string) (time.Time, error) {
	switch from {
	case "unix", "ms":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return time.Time{}, errors.New("Unix input must be a signed 64-bit decimal integer")
		}
		min, max := int64(minUnix), int64(maxUnix)
		if from == "ms" {
			min, max = min*1000, max*1000+999
		}
		if n < min || n > max {
			return time.Time{}, errors.New("time is outside the supported UTC year range 0000..9999")
		}
		if from == "ms" {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	case "rfc3339":
		if !rfc3339Syntax.MatchString(value) {
			return time.Time{}, errors.New("invalid RFC3339 time: include Z or a numeric offset and at most 9 fractional digits")
		}
		t, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return time.Time{}, errors.New("invalid RFC3339 date or time")
		}
		return t, nil
	default:
		return time.Time{}, errors.New("--from must be rfc3339, unix, or ms")
	}
}

func writeTime(stdout io.Writer, t time.Time, output, zone string) error {
	if year := t.UTC().Year(); year < 0 || year > 9999 {
		return errors.New("time is outside the supported UTC year range 0000..9999")
	}
	var result string
	switch output {
	case "unix":
		result = strconv.FormatInt(t.Unix(), 10)
	case "ms":
		result = strconv.FormatInt(t.UnixMilli(), 10)
	case "rfc3339":
		if zone == "" {
			return errors.New("time zone must not be empty")
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return errors.New("unknown time zone: use UTC, an IANA name, or Local")
		}
		t = t.In(location)
		if year := t.Year(); year < 0 || year > 9999 {
			return errors.New("time zone conversion exceeds the RFC3339 year range 0000..9999")
		}
		if _, offset := t.Zone(); offset%60 != 0 {
			return errors.New("time zone offset has seconds and cannot be represented exactly in RFC3339")
		}
		text, err := t.MarshalText()
		if err != nil {
			return err
		}
		result = string(text)
	}
	_, err := fmt.Fprintln(stdout, result)
	return err
}
