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
		opts.Expires = time.Duration(lifetimeSecs * int64(time.Second))
	})
	if err != nil {
		log.Printf("Couldn't get a presigned request to get %v:%v. Here's why: %v\n",
			bucketName, objectKey, err)
	}
	return request, err
}

var GenerateCfOutput = cli.Command{
	Name:  "generate-cf-output",
	Usage: "Generate cloudformation output",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "provider-id", Required: true, Usage: "publisher/name@version"},
		&cli.StringFlag{Name: "handler-id", Required: true, Usage: "The Id of the handler e.g aws-sso"},
		&cli.StringFlag{Name: "bootstrap-bucket", Required: true},
		&cli.StringFlag{Name: "stackname", Usage: "The name of the cloudformation stack"},
		&cli.StringFlag{Name: "commonfate-account-id", Usage: "The AWS account where Common Fate is deployed"},
		&cli.StringFlag{Name: "region", Usage: "The region to deploy the handler"},
		&cli.StringFlag{Name: "registry-api-url", Value: build.ProviderRegistryAPIURL, Hidden: true},
	},
	Action: func(c *cli.Context) error {
		ctx := c.Context
		bootstrapBucket := c.String("bootstrap-bucket")
		handlerId := c.String("handler-id")
		commonFateAWSAccountId := c.String("commonfate-account-id")
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
			var stackname = c.String("stackname")
			if stackname == "" {
				err = survey.AskOne(&survey.Input{Message: "enter the cloudformation stackname:", Default: handlerId}, &stackname)
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
			values["HandlerID"] = handlerId
			values["CommonFateAWSAccountId"] = commonFateAWSAccountId
			lambdaAssetPath := path.Join(provider.Publisher, provider.Name, provider.Version)
			values["AssetPath"] = path.Join(lambdaAssetPath, "handler.zip")

			awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
			if err != nil {
				return err
			}

			clio.Warn("Enter the values for your configurations:")
			for k, v := range res.JSON200.Schema.Config.AdditionalProperties {
				if v.Secret {

					client := ssm.NewFromConfig(awsCfg)

					var secret string
					name := createUniqueProviderSSMName(handlerId, k)
					helpMsg := fmt.Sprintf("This will be stored in aws system manager parameter store with name '%s'", name)
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

					clio.Successf("Added to AWS System Manager Parameter Store with name '%s'", name)

					values[ConvertToPascalCase(k)] = name

					continue
				}

				var v string
				err = survey.AskOne(&survey.Input{Message: k + ":"}, &v)
				if err != nil {
					return err
				}
				values[ConvertToPascalCase(k)] = v

			}

			if commonFateAWSAccountId == "" {
				var v string
				err = survey.AskOne(&survey.Input{Message: "enter the account Id of the account where commonfate is deployed:"}, &v)
				if err != nil {
					return err
				}
				values["CommonFateAWSAccountID"] = v
			}

			values["HandlerId"] = handlerId
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

// this will create a unique identifier for AWS System Manager Parameter Store
// for configuration field "api_url" this will result: 'publisher/provider-name/version/configuration/api_url'
func createUniqueProviderSSMName(handlerId string, k string) string {
	return "/" + path.Join("commonfate", "provider", handlerId, k)
}
