package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestCommandContextPropagatesCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	command := &cobra.Command{}
	command.SetContext(parent)

	ctx, cancel := commandContext(command)
	defer cancel()

	cancelParent()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("command context did not propagate parent cancellation")
	}
}
