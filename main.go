package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/andareed/polisee-tui/internal/policy"
	"github.com/andareed/polisee-tui/internal/tui"
)

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("polisee-tui", flag.ContinueOnError)
	flags.SetOutput(stderr)
	snapshot := flags.Bool("snapshot", false, "print one terminal frame without starting an interactive session")
	width := flags.Int("width", 120, "snapshot width in terminal cells")
	height := flags.Int("height", 40, "snapshot height in terminal rows")
	noMouse := flags.Bool("no-mouse", false, "leave mouse handling to the terminal")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: polisee-tui [options] [policy.xml]")
		fmt.Fprintln(stderr, "Without a file, opens the same synthetic demonstration as polisee-gui.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 {
		return fmt.Errorf("expected at most one policy XML path")
	}
	if *width < 1 || *height < 1 || *width > 500 || *height > 300 {
		return fmt.Errorf("snapshot size must be within 1–500 columns and 1–300 rows")
	}
	var doc *policy.Document
	var err error
	path := ""
	if flags.NArg() == 1 {
		path = tui.ExpandPath(flags.Arg(0))
		doc, err = policy.LoadFile(context.Background(), path)
	} else {
		doc, err = policy.LoadDemo(context.Background())
	}
	if err != nil {
		return fmt.Errorf("open policy: %w", err)
	}
	model := tui.New(doc)
	model.SetMouseEnabled(!*noMouse)
	if path != "" {
		model.StartPath(path)
	}
	if *snapshot {
		model.Update(tea.WindowSizeMsg{Width: *width, Height: *height})
		_, err = fmt.Fprintln(stdout, model.View().Content)
		return err
	}
	_, err = tea.NewProgram(model, tea.WithOutput(stdout)).Run()
	return err
}
func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "polisee-tui:", err)
		os.Exit(1)
	}
}
