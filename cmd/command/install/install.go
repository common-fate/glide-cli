package install

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmTypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/briandowns/spinner"
	"github.com/common-fate/cli/cmd/command"
	"github.com/common-fate/cli/cmd/middleware"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/cli/pkg/bootstrapper"
	"github.com/common-fate/cli/pkg/client"
	cfconfig "github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/cli/pkg/deployer"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	cftypes "github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	"github.com/urfave/cli/v2"
)

var Command = cli.Command{
	Name:        "install",
	Description: "Quickstart command to install a provider",
	Usage:       "Quickstart command to install a provider",
	Flags:       []cli.Flag{&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true}},
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

		bootstrapStackOutput, err := bs.Detect(ctx, false)
		if err == bootstrapper.ErrNotDeployed {
			clio.Debug("the bootstrap stack was not detected")
			clio.Warnf("To get started deploying providers, you need to bootstrap this AWS account and region (%s:%s)", awsContext.Account, awsContext.Config.Region)
			clio.Info("Bootstrapping will deploy a Cloudformation Stack then creates an S3 Bucket.\nProvider assets will be copied from the Common Fate Provider Registry into this bucket.\nThese assets can then be deployed into your account.")
			_, err := bs.Deploy(ctx, false)
			if err != nil {
				return err
			}
			bootstrapStackOutput, err = bs.Detect(ctx, true)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		clio.Debugw("completed bootstrapping", "bootstrap-output", bootstrapStackOutput)
		registryClient, err := providerregistrysdk.NewClientWithResponses(c.String("registry-api-url"))
		if err != nil {
			return errors.New("error configuring provider registry client")
		}

		// @TODO there should be an API which only returns the provider publisher and name combos
		// maybe just publisher
		// so the user can select by publisher -> name -> version
		//check that the provider type matches one in our registry
		res, err := registryClient.ListAllProvidersWithResponse(ctx)
		if err != nil {
			return err
		}
		var allProviders []providerregistrysdk.ProviderDetail
		switch res.StatusCode() {
		case http.StatusOK:
			allProviders = res.JSON200.Providers
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}

		var providers []string
		providerMap := map[string][]providerregistrysdk.ProviderDetail{}

		for _, provider := range allProviders {
			key := provider.Publisher + "/" + provider.Name
			providerMap[key] = append(providerMap[key], provider)
		}
		for k, v := range providerMap {
			providers = append(providers, k)
			// sort versions from newest to oldest
			sort.Slice(v, func(i, j int) bool {
				return v[i].Version > v[j].Version
			})
		}

		var selectedProviderType string
		p := &survey.Select{Message: "Select which provider you would like to deploy", Options: providers}
		err = survey.AskOne(p, &selectedProviderType)
		if err != nil {
			return err
		}

		var versions []string
		versionMap := map[string]providerregistrysdk.ProviderDetail{}
		for _, version := range providerMap[selectedProviderType] {
			versions = append(versions, version.Version)
			versionMap[version.Version] = version
		}

		var selectedProviderVersion string
		p = &survey.Select{Message: "Select which version of the provider you would like to deploy", Options: versions, Default: versions[0]} // sets the latest version as the default
		err = survey.AskOne(p, &selectedProviderVersion)
		if err != nil {
			return err
		}

		provider := versionMap[selectedProviderVersion]
		var kinds []string
		for kind := range *provider.Schema.Targets {
			kinds = append(kinds, kind)
		}

		var selectedProviderKind string
		p = &survey.Select{Message: "Select which Kind of target to use with this provider", Options: kinds, Default: kinds[0]} // sets the latest version as the default
		err = survey.AskOne(p, &selectedProviderKind)
		if err != nil {
			return err
		}

		// clio.Infof("You have selected to deploy: %s@%s and use the target Kind: %s", selectedProviderType, selectedProviderVersion, selectedProviderKind)
		clio.Info("Beginning to copy files from the registry to the bootstrap bucket")
		files, err := bs.CopyProviderFiles(ctx, provider)
		if err != nil {
			return err
		}
		clio.Success("Completed copying files from the registry to the bootstrap bucket")

		handlerID := strings.Join([]string{"cf-handler", provider.Publisher, provider.Name}, "-")
		lambdaAssetPath := path.Join(provider.Publisher, provider.Name, provider.Version)

		var parameters []types.Parameter

		config := provider.Schema.Config
		if config != nil {
			clio.Info("This Provider requires following configuration values to be configured.")
			clio.Info("Enter the value of these configurations:")
			for k, v := range *config {

				if v.Secret != nil && *v.Secret {
					client := ssm.NewFromConfig(awsContext.Config)

					var secret string
					name := command.SSMKey(command.SSMKeyOpts{
						HandlerID:    handlerID,
						Key:          k,
						Publisher:    provider.Publisher,
						ProviderName: provider.Name,
					})
					helpMsg := fmt.Sprintf("This will be stored in AWS SSM Parameter Store with name '%s'", name)
					err = survey.AskOne(&survey.Password{Message: k + ":", Help: helpMsg}, &secret)
					if err != nil {
						return err
					}

					_, err = client.PutParameter(ctx, &ssm.PutParameterInput{
						Name:      aws.String(name),
						Value:     aws.String(secret),
						Type:      ssmTypes.ParameterTypeSecureString,
						Overwrite: aws.Bool(true),
					})
					if err != nil {
						return err
					}

					clio.Successf("Added to AWS SSM Parameter Store with name '%s'", name)

					parameters = append(parameters, types.Parameter{
						ParameterKey:   aws.String(command.ConvertToPascalCase(k) + "Secret"),
						ParameterValue: aws.String(name),
					})

					continue
				}

				var v string
				err = survey.AskOne(&survey.Input{Message: k + ":"}, &v)
				if err != nil {
					return err
				}

				parameters = append(parameters, types.Parameter{
					ParameterKey:   aws.String(command.ConvertToPascalCase(k)),
					ParameterValue: aws.String(v),
				})
			}
		}

		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("CommonFateAWSAccountID"),
			ParameterValue: &awsContext.Account,
		})

		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("AssetPath"),
			ParameterValue: aws.String(path.Join(lambdaAssetPath, "handler.zip")),
		})

		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("BootstrapBucketName"),
			ParameterValue: aws.String(bootstrapStackOutput.AssetsBucket),
		})

		parameters = append(parameters, types.Parameter{
			ParameterKey:   aws.String("HandlerID"),
			ParameterValue: aws.String(handlerID),
		})

		// s3client := s3.NewFromConfig(awsContext.Config)
		// preSignedClient := s3.NewPresignClient(s3client)

		// presigner := command.Presigner{
		// 	PresignClient: preSignedClient,
		// }

		// req, err := presigner.GetObject(bootstrapStackOutput.AssetsBucket, path.Join(lambdaAssetPath, "cloudformation.json"), int64(time.Hour))
		// if err != nil {
		// 	return nil
		// }

		d, err := deployer.New(ctx)
		if err != nil {
			return err
		}

		clio.Infof("Deploying Cloudformation stack '%s'", handlerID)

		status, err := d.Deploy(ctx, files.CloudformationTemplateURL, parameters, nil, handlerID, "", true)
		if err != nil {
			return err
		}

		clio.Infof("Deployment completed with status '%s'", status)

		cfg, err := cfconfig.Load()
		if err != nil {
			return err
		}

		cf, err := client.FromConfig(ctx, cfg)
		if err != nil {
			return err
		}

		targetgroupID := strings.Join([]string{provider.Publisher, provider.Name}, "-")

		targetgroupRes, err := cf.AdminCreateTargetGroupWithResponse(ctx, cftypes.AdminCreateTargetGroupJSONRequestBody{
			Id:           targetgroupID,
			TargetSchema: provider.Publisher + "/" + provider.Name + "@" + provider.Version + "/" + selectedProviderKind,
		})
		if err != nil {
			return err
		}
		switch targetgroupRes.StatusCode() {
		case http.StatusCreated:
			clio.Successf("Successfully created the targetgroup: %s", targetgroupID)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}

		// register the targetgroup with handler

		reqBody := cftypes.AdminRegisterHandlerJSONRequestBody{
			AwsAccount: awsContext.Account,
			AwsRegion:  awsContext.Config.Region,
			Runtime:    "aws-lambda",
			Id:         handlerID,
		}

		rhr, err := cf.AdminRegisterHandlerWithResponse(ctx, reqBody)
		if err != nil {
			return err
		}

		switch rhr.StatusCode() {
		case http.StatusCreated:
			clio.Successf("Successfully registered handler '%s' with Common Fate", handlerID)
		default:
			return errors.New(string(rhr.Body))
		}

		tglr, err := cf.AdminCreateTargetGroupLinkWithResponse(ctx, targetgroupID, cftypes.AdminCreateTargetGroupLinkJSONRequestBody{
			DeploymentId: handlerID,
			Priority:     100,
			Kind:         selectedProviderKind,
		})
		if err != nil {
			return err
		}

		switch tglr.StatusCode() {
		case http.StatusOK:
			clio.Successf("Successfully linked Handler '%s' with Target Group '%s'", handlerID, targetgroupID)
		default:
			return errors.New(string(rhr.Body))
		}

		clio.Successf("TargetgroupId '%s' is successfully linked with the HandlerId '%s", targetgroupID, handlerID)
		si := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		si.Suffix = " waiting for Handler to become healthy"
		si.Writer = os.Stderr
		si.Start()
		defer si.Stop()
		var healthy bool
		for !healthy {
			ghr, err := cf.AdminGetHandlerWithResponse(ctx, handlerID)
			if err != nil {
				return err
			}
			switch rhr.StatusCode() {
			case http.StatusOK:
				if ghr.JSON200.Healthy {
					clio.Success("Your handler is now healthy")
					healthy = true
				}
			default:
				return errors.New(string(rhr.Body))
			}
		}
		return nil
	},
}
