package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"

	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/common-fate/pkg/service/targetsvc"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "provider",
	Description: "Prepare a provider from the registry for deployment into your account",
	Usage:       "Prepare a provider from the registry for deployment into your account",
	Subcommands: []*cli.Command{
		&BootstrapCommand,
		&ListCommand,
	},
}

var BootstrapCommand = cli.Command{
	Name:        "bootstrap",
	Description: "Bootstrapping a provider will clone the assets from the Common Fate registry to the bootstrap bucket in your account.",
	Usage:       "Bootstrapping a provider will clone the assets from the Common Fate registry to the bootstrap bucket in your account.",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "id", Required: true, Usage: "publisher/name@version"},
		&cli.StringFlag{Name: "bootstrap-bucket", Required: true, Aliases: []string{"bb"}, Usage: "The name of the bootstrap bucket to copy assets into", EnvVars: []string{"DEPLOYMENT_BUCKET"}},
		// &cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, EnvVars: []string{"COMMONFATE_PROVIDER_REGISTRY_API_URL"}, Hidden: true},
	},

	Action: func(c *cli.Context) error {

		ctx := context.Background()

		registryClient, err := providerregistrysdk.NewClientWithResponses(build.ProviderRegistryAPIURL)
		if err != nil {
			return errors.New("error configuring provider registry client")
		}

		provider, err := targetsvc.SplitProviderString(c.String("id"))
		if err != nil {
			return err
		}
		//check that the provider type matches one in our registry
		res, err := registryClient.GetProviderWithResponse(ctx, provider.Publisher, provider.Name, provider.Version)
		if err != nil {
			return err
		}
		switch res.StatusCode() {
		case http.StatusOK:
			clio.Success("Provider exists in the registry, beginning to clone assets.")
		case http.StatusNotFound:
			return errors.New(res.JSON404.Error)
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}

		//get bootstrap bucket

		//read from flag
		bootstrapBucket := c.String("bootstrap-bucket")

		//work out the lambda asset path
		lambdaAssetPath := path.Join(provider.Publisher, provider.Name, provider.Version)

		//copy the provider assets into the bucket (this will also copy the cloudformation template too)
		awsCfg, err := aws_config.LoadDefaultConfig(ctx)
		if err != nil {
			return err
		}
		client := s3.NewFromConfig(awsCfg)

		clio.Infof("Copying the handler.zip into %s", path.Join(bootstrapBucket, lambdaAssetPath, "handler.zip"))
		_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(bootstrapBucket),
			Key:        aws.String(path.Join(lambdaAssetPath, "handler.zip")),
			CopySource: aws.String(url.QueryEscape(res.JSON200.LambdaAssetS3Arn)),
		})
		if err != nil {
			return err
		}
		clio.Successf("Successfully copied the handler.zip into %s", path.Join(bootstrapBucket, lambdaAssetPath, "handler.zip"))

		clio.Infof("Copying the cloudformation template into %s", path.Join(bootstrapBucket, lambdaAssetPath, "cloudformation.json"))
		_, err = client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(bootstrapBucket),
			Key:        aws.String(path.Join(lambdaAssetPath, "cloudformation.json")),
			CopySource: aws.String(url.QueryEscape(res.JSON200.CfnTemplateS3Arn)),
		})
		if err != nil {
			return err
		}
		clio.Successf("Successfully copied the cloudformation template into %s", path.Join(bootstrapBucket, lambdaAssetPath, "cloudformation.json"))
		templateURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bootstrapBucket, awsCfg.Region, path.Join(lambdaAssetPath, "cloudformation.json"))
		clio.Info("Use the following cloudformation template URL to deploy this handler")
		clio.Info(templateURL)
		// clio.Infof("aws cloudformation create-stack --stack-name=<handler id> --template-url=%s --capabilities=CAPABILITY_IAM", templateURL)
		return nil
	},
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

		ctx := context.Background()
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
			table.SetHeader([]string{"name", "team", "version", "FunctionZipS3Path", "CFNTemplateS3Path"})
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
						d.Name, d.Publisher, d.Version, d.LambdaAssetS3Arn, d.CfnTemplateS3Arn,
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
