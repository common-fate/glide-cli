package authflow

import "errors"

// awsExports is the aws-exports.json file
// containing public client information
// in a Common Fate deployment.
type awsExports struct {
	Auth authExports `json:"Auth"`
	API  apiExports  `json:"API"`
}

// APIURL returns the API url of the aws-exports.json file.
// By default there is only 1 API endpoint defined in this file.
// Return an error if it does not exist
func (a awsExports) APIURL() (string, error) {
	if len(a.API.Endpoints) == 0 {
		return "", errors.New("common fate deployment has no API endpoints defined")
	}
	return a.API.Endpoints[0].Endpoint, nil
}

type oauthExports struct {
	Domain string `json:"domain"`
}

type authExports struct {
	Region         string       `json:"region"`
	UserPoolID     string       `json:"userPoolId"`
	CliAppClientID string       `json:"cliAppClientId"`
	Oauth          oauthExports `json:"oauth"`
}

type apiEndpoints struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Region   string `json:"region"`
}

type apiExports struct {
	Endpoints []apiEndpoints `json:"endpoints"`
}
