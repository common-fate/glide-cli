package bootstrapper

import (
	"context"
	"embed"
	"os"
	"time"

	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/briandowns/spinner"
	"github.com/common-fate/clio"
	"github.com/common-fate/cloudform/cfn"
	"github.com/common-fate/cloudform/console"
	"github.com/common-fate/cloudform/ui"
	"github.com/common-fate/common-fate/pkg/cfaws"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

//go:embed cloudformation
var cloudformationTemplates embed.FS

const BootstrapStackName = "CommonFateProviderAssetsBootstrapStack"

// check for bootstap bucket

// deploy bootstrap if required

// copy provider assets

// deploy provider with asset path

type BootstrapStackOutput struct {
	AssetsBucket string `json:"AssetsBucket"`
}

type Bootstrapper struct {
	cfnClient       *cloudformation.Client
	cloudformClient *cfn.Cfn
	s3Client        *s3.Client
	uiClient        *ui.UI
}

func New(ctx context.Context) (*Bootstrapper, error) {
	cfg, err := cfaws.ConfigFromContextOrDefault(ctx)
	if err != nil {
		return nil, err
	}

	return &Bootstrapper{
		cfnClient:       cloudformation.NewFromConfig(cfg),
		s3Client:        s3.NewFromConfig(cfg),
		cloudformClient: cfn.New(cfg),
		uiClient:        ui.New(cfg),
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

const noChangeFoundMsg = "The submitted information didn't contain changes. Submit different information to create a change set."

// Deploy deploys a stack and returns the final status
func (b *Bootstrapper) Deploy(ctx context.Context, confirm bool) (string, error) {

	template, err := cloudformationTemplates.ReadFile("cloudformation/bootstrap.json")
	if err != nil {
		return "", errors.Wrap(err, "error while loading template from embedded filesystem")
	}

	changeSetName, createErr := b.cloudformClient.CreateChangeSet(ctx, string(template), []types.Parameter{}, map[string]string{}, BootstrapStackName, "")

	si := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	si.Suffix = " creating CloudFormation change set"
	si.Writer = os.Stderr
	si.Start()
	si.Stop()

	if createErr != nil {
		if createErr.Error() == noChangeFoundMsg {
			clio.Success("Change set was created, but there is no change. Deploy was skipped.")
			return "DEPLOY_SKIPPED", nil
		} else {
			return "", errors.Wrap(createErr, "creating changeset")
		}
	}

	if !confirm {
		status, err := b.uiClient.FormatChangeSet(ctx, BootstrapStackName, changeSetName)
		if err != nil {
			return "", err
		}
		clio.Info("The following CloudFormation changes will be made:")
		fmt.Println(status)

		p := &survey.Confirm{Message: "Do you wish to continue?", Default: true}
		err = survey.AskOne(p, &confirm)
		if err != nil {
			return "", err
		}
		if !confirm {
			return "", errors.New("user cancelled deployment")
		}
	}

	err = b.cloudformClient.ExecuteChangeSet(ctx, BootstrapStackName, changeSetName)
	if err != nil {
		return "", err
	}

	status, messages := b.uiClient.WaitForStackToSettle(ctx, BootstrapStackName)

	fmt.Println("Final stack status:", ui.ColouriseStatus(status))

	if len(messages) > 0 {
		fmt.Println(console.Yellow("Messages:"))
		for _, message := range messages {
			fmt.Printf("  - %s\n", message)
		}
	}
	return status, nil
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
