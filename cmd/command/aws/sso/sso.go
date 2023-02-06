package sso

import "github.com/urfave/cli/v2"

var Command = cli.Command{
	Name:        "sso",
	Usage:       "Utilities for AWS IAM Identity Center (formerly AWS SSO) Access Providers",
	Subcommands: []*cli.Command{&generate, &populate},
}
