package cf

import (
	"github.com/common-fate/glide-cli/cmd/command"
	"github.com/common-fate/glide-cli/cmd/command/bootstrap"
	"github.com/common-fate/glide-cli/cmd/command/config"
	"github.com/common-fate/glide-cli/cmd/command/handler"
	"github.com/common-fate/glide-cli/cmd/command/provider"
	"github.com/common-fate/glide-cli/cmd/command/rules"
	"github.com/common-fate/glide-cli/cmd/command/targetgroup"
	"github.com/urfave/cli/v2"

	mw "github.com/common-fate/glide-cli/cmd/middleware"
)

var OSSSubCommand = cli.Command{
	Name:  "oss",
	Usage: "Actions for PDK providers",
	Subcommands: []*cli.Command{
		&command.Login,
		&command.Logout,
		&config.Command,
		&rules.Command,
		&provider.Command,
		&targetgroup.Command,
		&handler.Command,
		mw.WithBeforeFuncs(&bootstrap.Command, mw.RequireAWSCredentials()),
	},
}
