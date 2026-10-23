package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSnapshotAndCLIValidation(t *testing.T) {
	var output, errors bytes.Buffer
	if err := run([]string{"--snapshot", "--width", "100", "--height", "30"}, &output, &errors); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 30 {
		t.Fatalf("snapshot rows: %d", len(lines))
	}
	for _, line := range lines {
		if ansi.StringWidth(line) != 100 {
			t.Fatalf("snapshot width: %d", ansi.StringWidth(line))
		}
	}
	if !strings.Contains(ansi.Strip(output.String()), "Office access") {
		t.Fatal("default demo missing")
	}
	for _, args := range [][]string{{"--snapshot", "--width", "0"}, {"one.xml", "two.xml"}, {"--snapshot", "/no/such/synthetic-policy.xml"}} {
		if err := run(args, &output, &errors); err == nil {
			t.Fatalf("invalid CLI arguments accepted: %v", args)
		}
	}
	if err := run([]string{"--help"}, &output, &errors); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errors.String(), "Usage: polisee-tui") {
		t.Fatal("usage missing")
	}
}
