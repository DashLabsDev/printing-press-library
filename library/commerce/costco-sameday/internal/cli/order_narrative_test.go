package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestOrderPlaceRefusesWithoutGates(t *testing.T) {
	flags := &rootFlags{}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(newOrderNarrativeCmd(flags))
	root.SetArgs([]string{"order", "place", "--checkout-session-id", "sess_test"})
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected refusal without --yes/--confirm-charge")
	}
	if !strings.Contains(err.Error(), "refusing to charge") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "FinalizeCheckout") {
		t.Fatalf("preview missing FinalizeCheckout mention; out=%s", buf.String())
	}
}

func TestOrderPlaceDryRunNeverCharges(t *testing.T) {
	flags := &rootFlags{dryRun: true, yes: true}
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", true, "")
	root.PersistentFlags().BoolVar(&flags.yes, "yes", true, "")
	root.AddCommand(newOrderNarrativeCmd(flags))
	root.SetArgs([]string{"order", "place", "--checkout-session-id", "sess_test", "--confirm-charge", "--dry-run", "--yes"})
	flags.dryRun = true
	flags.yes = true
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	if err != nil {
		t.Fatalf("dry-run should succeed without network: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"dry_run": true`) && !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("expected dry_run true in output: %s", out)
	}
	if strings.Contains(out, `"charged": true`) || strings.Contains(out, `"charged":true`) {
		t.Fatalf("dry-run must not report charged true: %s", out)
	}
}

func TestOrderCancelRefusesWithoutYes(t *testing.T) {
	flags := &rootFlags{}
	root := &cobra.Command{Use: "root"}
	root.AddCommand(newOrderNarrativeCmd(flags))
	root.SetArgs([]string{"order", "cancel", "--order-id", "21349809721408208"})
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "refusing to cancel") {
		t.Fatalf("expected refuse without --yes, got %v", err)
	}
	if !strings.Contains(buf.String(), "/api/v2/orders/") {
		t.Fatalf("preview missing REST path: %s", buf.String())
	}
}

func TestOrderCancelDryRunNeverSends(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", true, "")
	root.AddCommand(newOrderNarrativeCmd(flags))
	root.SetArgs([]string{"order", "cancel", "--order-id", "21349809721408208", "--dry-run"})
	flags.dryRun = true
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	if err != nil {
		t.Fatalf("dry-run should succeed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"canceled": false`) && !strings.Contains(out, `"canceled":false`) {
		t.Fatalf("expected canceled false: %s", out)
	}
	if !strings.Contains(out, "NOT sent") {
		t.Fatalf("expected NOT sent wording: %s", out)
	}
}
