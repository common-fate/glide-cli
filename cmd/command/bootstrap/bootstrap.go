package bootstrap

import (
	"context"

	"github.com/common-fate/cli/pkg/bootstrapper"
	"github.com/common-fate/clio"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "bootstrap",
	Description: "Bootstrap a cloud account for deploying access providers",
	Usage:       "Bootstrap a cloud account for deploying access providers",

	Action: func(c *cli.Context) error {
		ctx := c.Context
		bucket, err := Bootstrap(ctx)
		if err != nil {
			return err
		}
		clio.Log(bucket)
		return nil
	},
}

func Bootstrap(ctx context.Context) (string, error) {
	bs, err := bootstrapper.New(ctx)
	if err != nil {
		return "", err
	}
	return bs.GetOrDeployBootstrapBucket(ctx)
}
