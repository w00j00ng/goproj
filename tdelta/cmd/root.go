package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/w00j00ng/goproj/tdelta/internal/compare"
	"github.com/w00j00ng/goproj/tdelta/internal/delta"
	"github.com/w00j00ng/goproj/tdelta/internal/render"
	"github.com/w00j00ng/goproj/tdelta/internal/viewer"
)

// Run returns 0 for success or equality, 1 when a comparison differs, and 2 for errors.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	code := 0
	root := &cobra.Command{
		Use:           "tdelta",
		Short:         "Compare text files and store or apply compressed deltas",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	for _, name := range []string{"create", "apply", "diff", "show"} {
		root.AddCommand(command(name, &code))
	}
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, render.Escape(err.Error(), false))
		return 2
	}
	return code
}

func command(name string, code *int) *cobra.Command {
	usage, description := "", ""
	switch name {
	case "create":
		usage, description = "create A B", "Save a compressed delta from A to B"
	case "apply":
		usage, description = "apply A DELTA", "Generate B from A and a saved delta"
	case "diff":
		usage, description = "diff A B", "Display changes between two text files"
	case "show":
		usage, description = "show A DELTA", "Display changes from A to the reconstructed B"
	}
	c := &cobra.Command{Use: usage, Short: description, Args: cobra.ExactArgs(2)}
	var output, editor, mode string
	var openEditor bool
	if name == "create" || name == "apply" {
		c.Flags().StringVarP(&output, "output", "o", "", "new output file (apply also accepts - for stdout)")
		c.MarkFlagRequired("output")
	} else {
		c.Flags().StringVar(&mode, "color", "auto", "color mode (auto, always, never)")
		c.Flags().BoolVar(&openEditor, "editor", false, "open the diff in the configured editor")
		c.Flags().StringVar(&editor, "editor-command", "", "editor command and flags")
	}

	c.RunE = func(c *cobra.Command, args []string) error {
		if args[0] == "-" && args[1] == "-" {
			return errors.New("only one input may read from stdin")
		}
		if c.Flags().Changed("editor-command") && !openEditor {
			return errors.New("--editor-command requires --editor")
		}
		if mode != "" && mode != "auto" && mode != "always" && mode != "never" || c.Flags().Changed("color") && mode == "" {
			return fmt.Errorf("invalid --color %q (use auto, always or never)", mode)
		}
		if (name == "create" || name == "apply") && (output == "" || (name == "create" && output == "-")) {
			return errors.New("--output must name a new file (only apply accepts -)")
		}
		a, err := read(c.InOrStdin(), args[0], compare.MaxText)
		if err != nil {
			return err
		}
		limit := compare.MaxText
		saved := name == "show" || name == "apply"
		if saved {
			limit = delta.MaxEncoded
		}
		b, err := read(c.InOrStdin(), args[1], limit)
		if err != nil {
			return err
		}
		if saved {
			b, err = delta.Apply(a, b)
			if err != nil {
				return err
			}
		}
		switch name {
		case "create":
			data, err := delta.Create(a, b)
			if err != nil {
				return err
			}
			return writeNew(output, data)
		case "apply":
			if output == "-" {
				return write(c.OutOrStdout(), b)
			}
			return writeNew(output, b)
		}

		changes, err := compare.Find(a, b)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			if openEditor {
				_, err = fmt.Fprintln(c.ErrOrStderr(), "No differences.")
			}
			return err
		}
		*code = 1
		from, to := args[0], args[1]
		if from == "-" {
			from = "A (stdin)"
		}
		if saved {
			to = "B"
		} else if to == "-" {
			to = "B (stdin)"
		}
		if openEditor {
			config, err := viewer.Configured(editor)
			if err != nil {
				return err
			}
			return viewer.Open(config, c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr(), func(w io.Writer) error {
				return render.Write(w, a, b, changes, from, to, false)
			})
		}
		return render.Write(c.OutOrStdout(), a, b, changes, from, to, useColor(c.OutOrStdout(), mode))
	}
	return c
}

func useColor(w io.Writer, mode string) bool {
	if mode == "always" {
		return true
	}
	if mode != "auto" || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	return ok && supportsColor(f)
}

func read(stdin io.Reader, path string, limit int) ([]byte, error) {
	var input io.Reader = stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		input = f
	}
	b, err := io.ReadAll(io.LimitReader(input, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if len(b) > limit {
		return nil, fmt.Errorf("input %q exceeds %d bytes", path, limit)
	}
	return b, nil
}

func write(w io.Writer, b []byte) error {
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

func writeNew(path string, b []byte) (err error) {
	dir, err := os.MkdirTemp(filepath.Dir(path), ".tdelta-")
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	defer func() {
		if cleanup := os.RemoveAll(dir); cleanup != nil {
			err = errors.Join(err, fmt.Errorf("clean temporary output: %w", cleanup))
		}
	}()
	staged := filepath.Join(dir, "output")
	f, err := os.OpenFile(staged, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if err := errors.Join(write(f, b), f.Close()); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	// Linking publishes a complete file without replacing an existing path.
	if err := os.Link(staged, path); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	return nil
}
