package provider

import (
	"errors"
	"net/http"
	"os"

	mw "github.com/common-fate/cli/cmd/middleware"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/common-fate/pkg/service/targetsvc"
	"github.com/common-fate/provider-registry-sdk-go/pkg/bootstrapper"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	registryclient "github.com/common-fate/provider-registry-sdk-go/pkg/registryclient"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "provider",
	Description: "Explore and manage Providers from the Provider Registry",
	Usage:       "Explore and manage Providers from the Provider Registry",
	Subcommands: []*cli.Command{
		&BootstrapCommand,
		&ListCommand,
		mw.WithBeforeFuncs(&installCommand, mw.RequireAWSCredentials()),
		mw.WithBeforeFuncs(&uninstallCommand, mw.RequireAWSCredentials()),
	},
}

var BootstrapCommand = cli.Command{
	Name:        "bootstrap",
	Description: "Before you can deploy a Provider, you will need to bootstrap it. This process will copy the files from the Provider Registry to your bootstrap bucket.",
	Usage:       "Copy a Provider into your AWS account",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true, Usage: "publisher/name@version"},
		&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true},
	},

	Action: func(c *cli.Context) error {
		ctx := c.Context

		registry, err := registryclient.New(ctx, registryclient.WithAPIURL(c.String("registry-api-url")))
		if err != nil {
			return err
		}

		provider, err := targetsvc.SplitProviderString(c.String("id"))
		if err != nil {
			return err
		}
		//check that the provider type matches one in our registry
		res, err := registry.GetProviderWithResponse(ctx, provider.Publisher, provider.Name, provider.Version)
		if err != nil {
			return err
		}

		clio.Success("Provider exists in the registry, beginning to clone assets.")

		awsContext, err := mw.AWSContextFromContext(ctx)
		if err != nil {
			return err
		}

		bs := bootstrapper.NewFromConfig(awsContext.Config)
		if err != nil {
			return err
		}
		_, err = bs.CopyProviderFiles(ctx, *res.JSON200)
		if err != nil {
			return err
		}

		return nil
	},
}

func getProviderId(publisher, name, version string) string {
	return publisher + "/" + name + "@" + version
}

var ListCommand = cli.Command{
	Name:        "list",
	Aliases:     []string{"ls"},
	Description: "List providers",
	Usage:       "List providers",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true},
	},
	Action: func(c *cli.Context) error {

		ctx := c.Context
		registryClient, err := providerregistrysdk.NewClientWithResponses(c.String("registry-api-url"))
		if err != nil {
			return errors.New("error configuring provider registry client")
		}

		//check that the provider type matches one in our registry
		res, err := registryClient.ListAllProvidersWithResponse(ctx)
		if err != nil {
			return err
		}

		switch res.StatusCode() {
		case http.StatusOK:
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"id", "name", "publisher", "version"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(true)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(true)

			if res.JSON200 != nil {
				for _, d := range res.JSON200.Providers {

					table.Append([]string{
						getProviderId(d.Publisher, d.Name, d.Version), d.Name, d.Publisher, d.Version,
					})
				}
			}

			table.Render()
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}
		return nil

	},
}
