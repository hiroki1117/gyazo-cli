package cli

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestHelpQuality checks that command help has what users and AI agents need
// to use the command from the help alone.
func TestHelpQuality(t *testing.T) {
	root := NewRootCmd(&App{})
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			if sub.Name() == "help" || sub.Name() == "completion" {
				continue
			}
			path := sub.CommandPath()
			if sub.Short == "" {
				t.Errorf("%s: Short is empty", path)
			}
			// Image leaf commands must describe their output and have examples.
			if !sub.HasSubCommands() && inGroup(sub, groupImage) {
				if !strings.Contains(strings.ToLower(sub.Long), "output") {
					t.Errorf("%s: Long must describe the output", path)
				}
				if sub.Example == "" {
					t.Errorf("%s: Example is empty", path)
				}
			}
			walk(sub)
		}
	}
	walk(root)
}

// inGroup reports whether the command or one of its ancestors is in one of the groups.
func inGroup(c *cobra.Command, groupIDs ...string) bool {
	for p := c; p != nil; p = p.Parent() {
		if slices.Contains(groupIDs, p.GroupID) {
			return true
		}
	}
	return false
}

func TestUnknownSubcommandFails(t *testing.T) {
	for _, group := range []string{"image", "auth", "config"} {
		r := runCmd(t, nil, nil, "", group, "no-such-command")
		if r.err == nil || !strings.Contains(r.err.Error(), "unknown command") {
			t.Errorf("%s: err = %v, want unknown command error", group, r.err)
		}
	}
}
