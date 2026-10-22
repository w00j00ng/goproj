package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"github.com/w00j00ng/goproj/portwho/internal/platform"
	"github.com/w00j00ng/goproj/portwho/internal/render"
)

var Version = "dev"

const usage = "Usage: portwho <port> [<port>...] [--family both|ipv4|ipv6] [--json|--pids] [--path]\n       portwho --all [--family both|ipv4|ipv6] [--json|--pids] [--path]\n"

func Execute() int { return Run(os.Args[1:], os.Stdout, os.Stderr, platform.New()) }

// Run executes the same CLI used by main, with injectable OS observations and writers.
func Run(args []string, out, errout io.Writer, provider inspect.Provider) int {
	stdout, stderr := &checkedWriter{Writer: out}, &checkedWriter{Writer: errout}
	var all, jsonMode, pids, includePath bool
	family := "both"
	code := 0
	c := &cobra.Command{Use: "portwho", Short: "Inspect local TCP listeners and observed process owners", Version: Version, SilenceUsage: true, SilenceErrors: true}
	c.SetOut(stdout)
	c.SetErr(stderr)
	c.SetArgs(args)
	c.SetVersionTemplate("portwho {{.Version}}\n")
	c.CompletionOptions.DisableDefaultCmd = true
	c.Flags().BoolVar(&all, "all", false, "Inspect all TCP LISTEN ports")
	c.Flags().StringVar(&family, "family", "both", "Socket family: both, ipv4, ipv6")
	c.Flags().BoolVar(&jsonMode, "json", false, "Output one JSON object (schema 2)")
	c.Flags().BoolVar(&pids, "pids", false, "Output sorted unique observed PIDs")
	c.Flags().BoolVar(&includePath, "path", false, "Include executable image paths")
	c.SetHelpFunc(func(_ *cobra.Command, _ []string) {
		fmt.Fprint(stdout, "Inspect local TCP LISTEN sockets; owners are best effort observations.\n\n"+usage+"\nOptions:\n  --all                Inspect all listening ports\n  --family both|ipv4|ipv6\n  --json               Output JSON schema 2\n  --pids               Output unique PIDs; skip image metadata\n  --path               Include executable image paths\n  -h, --help           Show help without OS queries\n  --version            Show version without OS queries\n\nExit codes: 0 found, 1 no match, 2 invalid usage, 3 incomplete query/output failure.\n")
	})
	c.RunE = func(_ *cobra.Command, ports []string) error {
		if family != "both" && family != "ipv4" && family != "ipv6" {
			return errors.New("--family must be both, ipv4, or ipv6")
		}
		if all && len(ports) > 0 {
			return errors.New("ports and --all cannot be combined")
		}
		if !all && len(ports) == 0 {
			return errors.New("provide one or more numeric ports or --all")
		}
		if jsonMode && pids {
			return errors.New("--json and --pids cannot be combined")
		}
		if includePath && pids {
			return errors.New("--path and --pids cannot be combined")
		}
		var normalized []uint16
		for _, s := range ports {
			if s == "" {
				return errors.New("ports must contain ASCII decimal digits from 1 to 65535")
			}
			for _, digit := range s {
				if digit < '0' || digit > '9' {
					return errors.New("ports must contain ASCII decimal digits from 1 to 65535")
				}
			}
			// Strip zeroes before parsing so arbitrarily padded valid ports remain decimal values.
			i := 0
			for i < len(s)-1 && s[i] == '0' {
				i++
			}
			port, err := strconv.ParseUint(s[i:], 10, 16)
			if err != nil || port == 0 {
				return errors.New("ports must be between 1 and 65535")
			}
			normalized = append(normalized, uint16(port))
		}
		slices.Sort(normalized)
		normalized = slices.Compact(normalized)
		query := inspect.Query{Ports: normalized, Family: family, IncludePath: includePath}
		var result inspect.Result
		result, code = inspect.Run(provider, query, pids)
		mode := "table"
		if jsonMode {
			mode = "json"
		}
		if pids {
			mode = "pids"
		}
		if err := render.Write(stdout, stderr, result, mode, code); err != nil {
			code = 3
			if stdout.err != nil && stderr.err == nil {
				fmt.Fprintln(stderr, "portwho: output failed")
			}
		}
		return nil
	}
	if err := c.Execute(); err != nil {
		code = 2
		fmt.Fprintf(stderr, "portwho: %s\n%s", err, usage)
	}
	if stdout.err != nil || stderr.err != nil {
		return 3
	}
	return code
}

type checkedWriter struct {
	io.Writer
	err error
}

func (w *checkedWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.Writer.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}
