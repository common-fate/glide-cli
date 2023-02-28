package main

import (
	"os"

	"github.com/common-fate/cli/cmd/command"
	"github.com/common-fate/cli/cmd/command/bootstrap"
	"github.com/common-fate/cli/cmd/command/config"
	"github.com/common-fate/cli/cmd/command/handler"
	"github.com/common-fate/cli/cmd/command/provider"
	"github.com/common-fate/cli/cmd/command/rules"
	"github.com/common-fate/cli/cmd/command/targetgroup"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      "cf",
		Writer:    os.Stderr,
		Usage:     "https://commonfate.io",
		UsageText: "cf [options] [command]",
		Version:   build.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "api-url", Usage: "override the Common Fate API URL"},
		},
		Commands: []*cli.Command{
			&command.Login,
			&command.Logout,
			&rules.Command,
			&bootstrap.Command,
			&provider.Command,
			&targetgroup.Command,
			&config.Command,
			&handler.Command,
			&command.GenerateCfOutput,
		},
	}
	clio.SetLevelFromEnv("CF_LOG")

	err := app.Run(os.Args)
	if err != nil {
		// if the error is an instance of clierr.PrintCLIErrorer then print the error accordingly
		if cliError, ok := err.(clierr.PrintCLIErrorer); ok {
			cliError.PrintCLIError()
		} else {
			clio.Error(err.Error())
		}
		os.Exit(1)
	}
}
