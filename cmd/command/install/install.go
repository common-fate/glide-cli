package install

import (
	"github.com/common-fate/cli/cmd/middleware"
	"github.com/common-fate/cli/pkg/bootstrapper"
	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "install",
	Description: "Quickstart command to install a provider",
	Usage:       "Quickstart command to install a provider",
	Action: func(c *cli.Context) error {
		ctx := c.Context
		bs, err := bootstrapper.New(ctx)
		if err != nil {
			return err
		}
		awsContext, err := middleware.AWSContextFromContext(ctx)
		if err != nil {
			return err
		}
		bootstrapStackOutput, err := bs.Detect(ctx)
		if err == bootstrapper.ErrNotDeployed {
			clio.Warnf("To get started deploying providers, you need to bootstrap this AWS account and region (%s:%s)", awsContext.Account, awsContext.Config.Region)
			clio.Info("Bootstrapping will deploy a Cloudformation Stack than creates an S3 Bucket.\nProvider assets will be copied from the Common Fate Provider Registry into this bucket.\nThese assets can then be deployed into your account.")
			_, err := bs.Deploy(ctx, false)
			if err != nil {
				return err
			}
			bootstrapStackOutput, err = bs.Detect(ctx)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		clio.Debugw("completed bootstrapping", "bootstrap-output", bootstrapStackOutput)

		return nil
	},
}
