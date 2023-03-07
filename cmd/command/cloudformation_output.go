package command

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/common-fate/cli/internal/build"
	"github.com/common-fate/clio"
	"github.com/common-fate/clio/clierr"
	"github.com/common-fate/common-fate/pkg/service/targetsvc"
	"github.com/common-fate/provider-registry-sdk-go/pkg/providerregistrysdk"
	"github.com/urfave/cli/v2"
)

type Presigner struct {
	PresignClient *s3.PresignClient
}

// GetObject makes a presigned request that can be used to get an object from a bucket.
// The presigned request is valid for the specified number of seconds.
func (presigner Presigner) GetObject(
	bucketName string, objectKey string, lifetimeSecs int64) (*v4.PresignedHTTPRequest, error) {
	request, err := presigner.PresignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(lifetimeSecs)
	})
	if err != nil {
		log.Printf("Couldn't get a presigned request to get %v:%v. Here's why: %v\n",
			bucketName, objectKey, err)
	}
	return request, err
}

var GenerateCfOutput = cli.Command{
	Name:  "cloudformation",
	Usage: "Manage CloudFormation templates for Providers",
	Subcommands: []*cli.Command{
		&cfnCommandCommand,
	},
}

var cfnCommandCommand = cli.Command{
	Name:  "command",
	Usage: "Generate AWS CLI commands to create or update CloudFormation stacks",
	Subcommands: []*cli.Command{
		&UpdateStack,
		&CreateStack,
	},
}

func ConvertToPascalCase(s string) string {
	arg := strings.Split(s, "_")
	var formattedStr []string

	for _, v := range arg {
		formattedStr = append(formattedStr, strings.ToUpper(v[0:1])+v[1:])
	}

	return strings.Join(formattedStr, "")
}

func convertValuesToCloudformationParameter(m map[string]string) string {
	parameters := "--parameters "

	for k, v := range m {
		parameters = parameters + strings.Join([]string{fmt.Sprintf("ParameterKey=\"%s\"", k), ",", fmt.Sprintf("ParameterValue=\"%s\"", v)}, "") + " "
	}

	return parameters
}

var CreateStack = cli.Command{
	Name:  "create",
	Usage: "Generate an 'aws cloudformation create-stack' command",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "provider-id", Required: true, Usage: "publisher/name@version"},
		&cli.StringFlag{Name: "handler-id", Required: true, Usage: "The ID of the Handler (for example, 'cf-handler-aws')"},
		&cli.StringFlag{Name: "bootstrap-bucket", Required: true},
		&cli.StringFlag{Name: "common-fate-account-id", Usage: "The AWS account where Common Fate is deployed"},
		&cli.StringFlag{Name: "region", Usage: "The region to deploy the handler"},
		&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context
		bootstrapBucket := c.String("bootstrap-bucket")
		handlerID := c.String("handler-id")
		commonFateAWSAccountID := c.String("common-fate-account-id")
		registryClient, err := providerregistrysdk.NewClientWithResponses(c.String("registry-api-url"))
		if err != nil {
			return errors.New("error configuring provider registry client")
		}

		provider, err := targetsvc.SplitProviderString(c.String("provider-id"))
		if err != nil {
			return err
		}

		// check that the provider type matches one in our registry
		res, err := registryClient.GetProviderWithResponse(ctx, provider.Publisher, provider.Name, provider.Version)
		if err != nil {
			return err
		}

		if res.JSON500 != nil {
			return errors.New("unable to ping the registry client: " + res.JSON500.Error)
		}

		switch res.StatusCode() {
		case http.StatusOK:
			var stackname = c.String("handler-id")
			if stackname == "" {
				err = survey.AskOne(&survey.Input{Message: "enter the cloudformation stackname:", Default: handlerID}, &stackname)
				if err != nil {
					return err
				}
			}

			var region = c.String("region")
			if region == "" {
				err = survey.AskOne(&survey.Input{Message: "enter the region of cloudformation stack deployment"}, &region)
				if err != nil {
					return err
				}
			}

			values := make(map[string]string)

			values["BootstrapBucketName"] = bootstrapBucket
			values["HandlerID"] = handlerID
			values["CommonFateAWSAccountID"] = commonFateAWSAccountID
			lambdaAssetPath := path.Join(provider.Publisher, provider.Name, provider.Version)
			values["AssetPath"] = path.Join(lambdaAssetPath, "handler.zip")

			awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
			if err != nil {
				return err
			}

			config := res.JSON200.Schema.Config
			if config != nil {
				clio.Info("Enter the values for your configurations:")
				for k, v := range *config {
					if v.Secret != nil && *v.Secret {
						client := ssm.NewFromConfig(awsCfg)

						var secret string
						name := ssmKey(ssmKeyOpts{
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
							Type:      types.ParameterTypeSecureString,
							Overwrite: aws.Bool(true),
						})
						if err != nil {
							return err
						}

						clio.Successf("Added to AWS SSM Parameter Store with name '%s'", name)

						// secret config should have "Secret" prefix to the config key name.
						values[ConvertToPascalCase(k)+"Secret"] = name

						continue
					}

					var v string
					err = survey.AskOne(&survey.Input{Message: k + ":"}, &v)
					if err != nil {
						return err
					}
					values[ConvertToPascalCase(k)] = v

				}
			}

			if commonFateAWSAccountID == "" {
				var v string
				err = survey.AskOne(&survey.Input{Message: "The ID of the AWS account where Common Fate is deployed:"}, &v)
				if err != nil {
					return err
				}
				values["CommonFateAWSAccountID"] = v
			}

			parameterKeys := convertValuesToCloudformationParameter(values)

			s3client := s3.NewFromConfig(awsCfg)
			preSignedClient := s3.NewPresignClient(s3client)

			presigner := Presigner{
				PresignClient: preSignedClient,
			}

			req, err := presigner.GetObject(bootstrapBucket, path.Join(lambdaAssetPath, "cloudformation.json"), int64(time.Hour))
			if err != nil {
				return nil
			}

			templateUrl := fmt.Sprintf(" --template-url \"%s\" ", req.URL)
			stackNameFlag := fmt.Sprintf(" --stack-name %s ", stackname)
			regionFlag := fmt.Sprintf(" --region %s ", region)

			output := strings.Join([]string{"aws cloudformation create-stack", stackNameFlag, regionFlag, templateUrl, parameterKeys, "--capabilities CAPABILITY_NAMED_IAM"}, "")

			fmt.Printf("%v \n", output)

		case http.StatusNotFound:
			return errors.New(res.JSON404.Error)
		case http.StatusInternalServerError:
			return errors.New(res.JSON500.Error)
		default:
			return clierr.New("Unhandled response from the Common Fate API", clierr.Infof("Status Code: %d", res.StatusCode()), clierr.Error(string(res.Body)))
		}

		return nil
	},
}

var UpdateStack = cli.Command{
	Name:  "update",
	Usage: "Generate an 'aws cloudformation update-stack' command",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "handler-id", Usage: "The Handler ID and name of the CloudFormation stack", Required: true},
		&cli.StringFlag{Name: "region", Usage: "The region to deploy the handler", Required: true},
	},
	Action: func(c *cli.Context) error {
		stackname := c.String("handler-id")
		region := c.String("region")

		ctx := c.Context

		awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return err
		}

		cfn := cloudformation.NewFromConfig(awsCfg)

		out, err := cfn.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
			StackName: &stackname,
		})
		if err != nil {
			return err
		}

		if len(out.Stacks) > 0 {
			stack := out.Stacks[0]

			values := make(map[string]string)
			for _, parameter := range stack.Parameters {

				// secret values have this prefix so need to update the SSM parameter store for these keys
				if strings.HasPrefix(*parameter.ParameterValue, "awsssm:///common-fate/provider/") {
					var shouldUpdate bool

					err = survey.AskOne(&survey.Confirm{Message: "Do you want to update value for " + *parameter.ParameterKey + " in AWS parameter store?"}, &shouldUpdate)
					if err != nil {
						return err
					}

					if shouldUpdate {
						client := ssm.NewFromConfig(awsCfg)

						var secret string
						name := *parameter.ParameterValue
						helpMsg := fmt.Sprintf("This will be stored in aws system manager parameter store with name '%s'", name)
						err = survey.AskOne(&survey.Password{Message: *parameter.ParameterKey + ":", Help: helpMsg}, &secret)
						if err != nil {
							return err
						}

						_, err = client.PutParameter(ctx, &ssm.PutParameterInput{
							Name:      aws.String(name),
							Value:     aws.String(secret),
							Type:      types.ParameterTypeSecureString,
							Overwrite: aws.Bool(true),
						})
						if err != nil {
							return err
						}

						clio.Successf("Updated value in AWS System Manager Parameter Store for key with name '%s'", name)
					}

					continue
				}

				var v string
				err = survey.AskOne(&survey.Input{Message: *parameter.ParameterKey + ":", Default: *parameter.ParameterValue}, &v)
				if err != nil {
					return err
				}

				if v != *parameter.ParameterValue {
					values[*parameter.ParameterKey] = v
				} else {
					values[*parameter.ParameterKey] = *parameter.ParameterValue
				}
			}

			parameterKeys := convertValuesToCloudformationParameter(values)

			s3client := s3.NewFromConfig(awsCfg)
			preSignedClient := s3.NewPresignClient(s3client)

			presigner := Presigner{
				PresignClient: preSignedClient,
			}

			bootstrapBucket := values["BootstrapBucketName"]
			lambdaAssetPath := values["AssetPath"]

			req, err := presigner.GetObject(bootstrapBucket, strings.Replace(lambdaAssetPath, "handler.zip", "cloudformation.json", 1), int64(time.Hour)*1)
			if err != nil {
				return nil
			}

			templateUrl := fmt.Sprintf(" --template-url \"%s\" ", req.URL)
			stackNameFlag := fmt.Sprintf(" --stack-name %s ", stackname)
			regionFlag := fmt.Sprintf(" --region %s ", region)

			output := strings.Join([]string{"aws cloudformation update-stack", stackNameFlag, regionFlag, templateUrl, parameterKeys, "--capabilities CAPABILITY_NAMED_IAM"}, "")

			fmt.Printf("%v \n", output)
		}

		return nil
	},
}

type ssmKeyOpts struct {
	HandlerID    string
	Key          string
	Publisher    string
	ProviderName string
}

// this will create a unique identifier for AWS System Manager Parameter Store
// for configuration field "api_url" this will result: 'publisher/provider-name/version/configuration/api_url'
func ssmKey(opts ssmKeyOpts) string {
	return "awsssm:///" + path.Join("common-fate", "provider", opts.Publisher, opts.ProviderName, opts.HandlerID, opts.Key)
}
