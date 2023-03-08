package provider

import (
	"strings"

	"github.com/common-fate/cli/cmd/middleware"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/cli/pkg/client"
	cfconfig "github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/common-fate/cloudform/deployer"
	"github.com/common-fate/provider-registry-sdk-go/pkg/handlerclient"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	"github.com/urfave/cli/v2"
)

var uninstallCommand = cli.Command{
	Name:        "uninstall",
	Description: "Quickstart all-in-one command to remove a provider",
	Usage:       "Quickstart all-in-one command to remove a provider",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true},
		&cli.StringFlag{Name: "handler-id", Usage: "The Handler ID to remove", Required: true},
		&cli.StringFlag{Name: "target-group-id", Usage: "Override the ID of the Target Group which will be deleted"},
		&cli.BoolFlag{Name: "delete-cloudformation-stack", Usage: "Delete the CloudFormation stack for the Handler", Value: true},
		&cli.BoolFlag{Name: "confirm", Aliases: []string{"y"}, Usage: "Confirm the deletion of resources"},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context

		awsContext, err := middleware.AWSContextFromContext(ctx)
		if err != nil {
			return err
		}

		cfg, err := cfconfig.Load()
		if err != nil {
			return err
		}

		// the client needs to be constructed as early as possible in the
		// CLI command, because client.FromConfig() returns an error
		// prompting the user to run 'cf login' if they are unauthenticated.
		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		// make an admin API call. Even though we don't use the response,
		// this will cause the CLI wizard to fail early if the auth token
		// is expired, or if the user is not an administrator.
		_, err = cf.AdminListHandlersWithResponse(ctx)
		if err != nil {
			return err
		}

		handlerID := c.String("handler-id")

		providerRuntime, err := handlerclient.NewLambdaRuntime(c.Context, handlerID)
		if err != nil {
			return err
		}

		var desc *providerregistrysdk.DescribeResponse

		desc, err = providerRuntime.Describe(c.Context)
		if err != nil {
			// log errors but still continue, as the handler may be in a totally invalid
			// state which prevents us from calling the Describe API
			clio.Errorf("Error when describing Handler Lambda function (continuing with deletion anyway): %s", err.Error())
		}

		d := deployer.NewFromConfig(awsContext.Config)

		if c.Bool("delete-cloudformation-stack") {
			clio.Infof("Deleting CloudFormation stack '%s'", handlerID)

			_, err = d.Delete(ctx, deployer.DeleteOpts{
				StackName: handlerID,
			})
			if err != nil {
				return err
			}
		}

		targetgroupID := c.String("target-group-id")
		if targetgroupID == "" {
			targetgroupID = strings.TrimPrefix(handlerID, "cf-handler-")
		}

		clio.Infof("Deleting Target Group '%s'", targetgroupID)
		_, err = cf.AdminDeleteTargetGroupWithResponse(ctx, targetgroupID)
		if err != nil {
			return err
		}

		clio.Infof("Deleting Handler '%s'", targetgroupID)
		_, err = cf.AdminDeleteHandlerWithResponse(ctx, handlerID)
		if err != nil {
			return err
		}

		clio.Successf("Handler '%s' has been removed", handlerID)
		if desc != nil {
			clio.Infof("You can deploy this handler again by running:\ncf provider install -p %s --handler-id %s", desc.Provider, handlerID)
		}

		return nil
	},
}
