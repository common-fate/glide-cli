package deployer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/briandowns/spinner"
	"github.com/common-fate/clio"
	"github.com/common-fate/cloudform/cfn"
	"github.com/common-fate/cloudform/console"
	"github.com/common-fate/cloudform/ui"
	"github.com/pkg/errors"
)

type Deployer struct {
	cfnClient       *cloudformation.Client
	cloudformClient *cfn.Cfn
	uiClient        *ui.UI
}

func New(ctx context.Context) (*Deployer, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &Deployer{
		cfnClient:       cloudformation.NewFromConfig(cfg),
		cloudformClient: cfn.New(cfg),
		uiClient:        ui.New(cfg),
	}, nil
}

// NewFromConfig creates a Deployer from an existing AWS config.
func NewFromConfig(cfg aws.Config) *Deployer {
	return &Deployer{
		cfnClient:       cloudformation.NewFromConfig(cfg),
		cloudformClient: cfn.New(cfg),
		uiClient:        ui.New(cfg),
	}
}

const noChangeFoundMsg = "The submitted information didn't contain changes. Submit different information to create a change set."

// Deploy deploys a stack and returns the final status
// template can be either a URL or a template body
func (b *Deployer) Deploy(ctx context.Context, template string, params []types.Parameter, tags map[string]string, stackName string, roleArn string, confirm bool) (string, error) {
	changeSetName, createErr := b.cloudformClient.CreateChangeSet(ctx, string(template), params, tags, stackName, "")

	si := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	si.Suffix = " creating CloudFormation change set"
	si.Writer = os.Stderr
	si.Start()
	si.Stop()

	if createErr != nil {
		if createErr.Error() == noChangeFoundMsg {
			clio.Info("Skipped deployment (there are no changes in the changeset)")
			return "DEPLOY_SKIPPED", nil
		} else {
			return "", errors.Wrap(createErr, "creating changeset")
		}
	}

	if !confirm {
		status, err := b.uiClient.FormatChangeSet(ctx, stackName, changeSetName)
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

	err := b.cloudformClient.ExecuteChangeSet(ctx, stackName, changeSetName)
	if err != nil {
		return "", err
	}

	status, messages := b.uiClient.WaitForStackToSettle(ctx, stackName)

	clio.Infof("Final stack status: %s", ui.ColouriseStatus(status))

	if len(messages) > 0 {
		fmt.Println(console.Yellow("Messages:"))
		for _, message := range messages {
			fmt.Printf("  - %s\n", message)
		}
	}
	return status, nil
}

// Delete a CloudFormation stack and returns the final status
func (b *Deployer) Delete(ctx context.Context, stackName string, roleArn string) (string, error) {
	_, err := b.cloudformClient.DeleteStack(stackName, roleArn)
	if err != nil {
		return "", err
	}

	si := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	si.Suffix = " creating CloudFormation change set"
	si.Writer = os.Stderr
	si.Start()
	si.Stop()

	status, messages := b.uiClient.WaitForStackToSettle(ctx, stackName)

	clio.Infof("Final stack status: %s", ui.ColouriseStatus(status))

	if len(messages) > 0 {
		fmt.Println(console.Yellow("Messages:"))
		for _, message := range messages {
			fmt.Printf("  - %s\n", message)
		}
	}
	return status, nil
}
