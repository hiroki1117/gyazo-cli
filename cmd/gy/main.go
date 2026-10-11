// Command gy is a command line client for Gyazo.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/hiroki1117/gyazo-cli/internal/cli"
)

// Set by -ldflags "-X main.version=..." in release builds.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	app := &cli.App{
		IO:        cli.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		Getenv:    os.Getenv,
		BuildInfo: buildInfo(),
	}
	if err := cli.NewRootCmd(app).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if hint := cli.ErrorHint(err); hint != "" {
			fmt.Fprintln(os.Stderr, hint)
		}
		return 1
	}
	return 0
}

// buildInfo fills values not set by ldflags from the module and VCS info recorded by Go.
func buildInfo() cli.BuildInfo {
	bi := cli.BuildInfo{Version: version, Commit: commit, Date: date}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return bi
	}
	if bi.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		bi.Version = info.Main.Version
	}
	for _, s := range info.Settings {
		switch {
		case s.Key == "vcs.revision" && bi.Commit == "none":
			bi.Commit = s.Value
		case s.Key == "vcs.time" && bi.Date == "unknown":
			bi.Date = s.Value
		}
	}
	return bi
}
