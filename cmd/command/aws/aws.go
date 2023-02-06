package aws

import (
	"github.com/common-fate/cli/cmd/command/aws/sso"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "aws",
	Subcommands: []*cli.Command{&sso.Command},
	Usage:       "Utilities for AWS Access Providers",
}
