package bootstrapper

import (
	"context"
	"embed"
	"net/url"
	"path"

	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/common-fate/cli/pkg/deployer"
	"github.com/common-fate/clio"
	"github.com/common-fate/common-fate/pkg/cfaws"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

//go:embed cloudformation
var cloudformationTemplates embed.FS

const BootstrapStackName = "CommonFateProviderAssetsBootstrapStack"

type BootstrapStackOutput struct {
	AssetsBucket string `json:"AssetsBucket"`
}

type Bootstrapper struct {
	cfnClient *cloudformation.Client
	s3Client  *s3.Client
	deployer  *deployer.Deployer
}

func New(ctx context.Context) (*Bootstrapper, error) {
	cfg, err := cfaws.ConfigFromContextOrDefault(ctx)
	if err != nil {
		return nil, err
	}

	deploy, err := deployer.New(ctx)
	if err != nil {
		return nil, err
	}
	return &Bootstrapper{
		cfnClient: cloudformation.NewFromConfig(cfg),
		s3Client:  s3.NewFromConfig(cfg),
		deployer:  deploy,
	}, nil
}

var ErrNotDeployed error = errors.New("bootstrap stack has not yet been deployed in this account and region")

func (b *Bootstrapper) Detect(ctx context.Context) (*BootstrapStackOutput, error) {
	stacks, err := b.cfnClient.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(BootstrapStackName),
	})
	var genericError *smithy.GenericAPIError
	if ok := errors.As(err, &genericError); ok && genericError.Code == "ValidationError" {
		return nil, ErrNotDeployed
	} else if err != nil {
		return nil, err
	} else if len(stacks.Stacks) != 1 {
		return nil, fmt.Errorf("expected 1 stack but got %d", len(stacks.Stacks))
	}

	return ProcessOutputs(stacks.Stacks[0])
}

func ProcessOutputs(stack types.Stack) (*BootstrapStackOutput, error) {
	// decode the output variables into the Go struct.
	outputMap := make(map[string]string)
	for _, o := range stack.Outputs {
		outputMap[*o.OutputKey] = *o.OutputValue
	}

	var out BootstrapStackOutput
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &out})
	if err != nil {
		return nil, errors.Wrap(err, "setting up decoder")
	}
	err = decoder.Decode(outputMap)
	if err != nil {
		return nil, errors.Wrap(err, "decoding CloudFormation outputs")
	}
	return &out, nil

}

// Deploy deploys a stack and returns the final status
func (b *Bootstrapper) Deploy(ctx context.Context, confirm bool) (string, error) {

	template, err := cloudformationTemplates.ReadFile("cloudformation/bootstrap.json")
	if err != nil {
		return "", errors.Wrap(err, "error while loading template from embedded filesystem")
	}

	return b.deployer.Deploy(ctx, string(template), []types.Parameter{}, map[string]string{}, BootstrapStackName, "", true)
}

// GetOrDeployBootstrap loads the output if the stack already exists, else it deploys the bootstrap stack first
func (b *Bootstrapper) GetOrDeployBootstrapBucket(ctx context.Context) (string, error) {
	out, err := b.Detect(ctx)
	if err == ErrNotDeployed {
		_, err := b.Deploy(ctx, true)
		if err != nil {
			return "", err
		}
		out, err := b.Detect(ctx)
		if err != nil {
			return "", err
		}
		return out.AssetsBucket, nil
	}
	if err != nil {
		return "", err
	}
	return out.AssetsBucket, nil
}

// CopyProviderFiles will clone the handler and cfn template from the registry bucket to the bootstrap bucket of the current account
func (b *Bootstrapper) CopyProviderFiles(ctx context.Context, provider providerregistrysdk.ProviderDetail) error {
	// detect the bootstrap bucket
	out, err := b.Detect(ctx)
	if err != nil {
		return err
	}

	lambdaAssetPath := path.Join(provider.Publisher, provider.Name, provider.Version)
	clio.Infof("Copying the handler.zip into %s", path.Join(out.AssetsBucket, lambdaAssetPath, "handler.zip"))
	_, err = b.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(out.AssetsBucket),
		Key:        aws.String(path.Join(lambdaAssetPath, "handler.zip")),
		CopySource: aws.String(url.QueryEscape(provider.LambdaAssetS3Arn)),
	})
	if err != nil {
		return err
	}
	clio.Successf("Successfully copied the handler.zip into %s", path.Join(out.AssetsBucket, lambdaAssetPath, "handler.zip"))

	clio.Infof("Copying the cloudformation template into %s", path.Join(out.AssetsBucket, lambdaAssetPath, "cloudformation.json"))
	_, err = b.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(out.AssetsBucket),
		Key:        aws.String(path.Join(lambdaAssetPath, "cloudformation.json")),
		CopySource: aws.String(url.QueryEscape(provider.CfnTemplateS3Arn)),
	})
	clio.Successf("Successfully copied the cloudformation template into %s", path.Join(out.AssetsBucket, lambdaAssetPath, "cloudformation.json"))
	return err
}
