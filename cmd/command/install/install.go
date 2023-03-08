package install

import (
	"errors"
	"net/http"
	"sort"

	"github.com/AlecAivazis/survey/v2"
	"github.com/common-fate/cli/cmd/middleware"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/cli/pkg/bootstrapper"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
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

		bootstrapStackOutput, err := bs.Detect(ctx)
		if err == bootstrapper.ErrNotDeployed {
			clio.Debug("the bootstrap stack was not detected")
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

		clio.Infof("You have selected to deploy: %s@%s and use the target Kind: %s", selectedProviderType, selectedProviderVersion, selectedProviderKind)
		clio.Info("Beginning to copy files from the registry to the bootstrap bucket")
		err = bs.CopyProviderFiles(ctx, provider)
		if err != nil {
			return err
		}
		clio.Success("Completed copying files from the registry to the bootstrap bucket")

		return nil
	},
}
