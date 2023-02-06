// Package authflow handles the OAuth2.0 flow against Cognito.
package authflow

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/common-fate/cli/pkg/config"
	"github.com/common-fate/clio"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// Response contains authenticated user information
// after the OAuth2.0 login flow is complete.
type Response struct {
	// Err is set if there was an error which
	// prevented the flow from completing
	Err     error
	Token   *oauth2.Token
	Context config.Context
}

type Server struct {
	response chan Response
	context  config.Context
}

func NewServer(userInfo chan Response) *Server {
	return &Server{
		response: userInfo,
	}
}

type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Opts struct {
	// Response is the channel where the auth response will be
	// sent upon successful authentication.
	Response chan Response

	// DashboardURL is the web dashboard URL
	DashboardURL string

	// Client is an optional HTTP client which can be specified.
	// If not provided defaults to http.DefaultClient.
	Client Doer
}

// FromDashboardURL builds a local server for an OAuth2.0 login flow
// looking up the CLI Client ID from the deployment public exports endpoint.
func FromDashboardURL(ctx context.Context, opts Opts) (*Server, error) {
	if opts.Client == nil {
		opts.Client = http.DefaultClient
	}

	u, err := url.Parse(opts.DashboardURL)
	if err != nil {
		return nil, errors.Wrap(err, "parsing dashboard URL")
	}
	u.Path = path.Join(u.Path, "aws-exports.json")

	// fetch the aws-exports.json file containing the public app client info
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "building deployment exports request")
	}

	res, err := opts.Client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "making deployment exports request")
	}

	var exp awsExports
	err = json.NewDecoder(res.Body).Decode(&exp)
	if err != nil {
		return nil, errors.Wrap(err, "decoding deployment exports")
	}

	cognitoURL := url.URL{
		Scheme: "https",
		Host:   exp.Auth.Oauth.Domain,
	}

	authURL := cognitoURL
	authURL.Path = "/oauth2/authorize"

	tokenURL := cognitoURL
	tokenURL.Path = "/oauth2/token"

	apiURL, err := exp.APIURL()
	if err != nil {
		return nil, err
	}

	depCtx := config.Context{
		AuthURL:      authURL.String(),
		TokenURL:     tokenURL.String(),
		DashboardURL: opts.DashboardURL,
		ClientID:     exp.Auth.CliAppClientID,
		APIURL:       apiURL,
	}

	s := Server{
		response: opts.Response,
		context:  depCtx,
	}

	return &s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/auth/cognito/login", s.oauthLogin)
	mux.HandleFunc("/auth/cognito/callback", s.oauthCallback)

	return mux
}

func (s *Server) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		RedirectURL: "http://localhost:18900/auth/cognito/callback",
		ClientID:    s.context.ClientID,
		Scopes:      []string{"openid", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  s.context.AuthURL,
			TokenURL: s.context.TokenURL,
		},
	}
}

func (s *Server) oauthLogin(w http.ResponseWriter, r *http.Request) {
	// Create oauthState cookie
	oauthState := generateStateOauthCookie(w)

	/*
		AuthCodeURL receive state that is a token to protect the user from CSRF attacks. You must always provide a non-empty string and
		validate that it matches the the state query parameter on your redirect callback.
	*/
	u := s.oauthConfig().AuthCodeURL(oauthState)

	http.Redirect(w, r, u, http.StatusTemporaryRedirect)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	// Read oauthState from Cookie
	oauthState, _ := r.Cookie("oauthstate")

	if r.FormValue("state") != oauthState.Value {
		log.Println("invalid oauth state")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	data, err := s.getUserData(r.FormValue("code"))
	if err != nil {
		log.Println(err.Error())
		s.response <- Response{Err: err}

		w.Write([]byte("there was a problem logging in to Common Fate: " + err.Error()))
		w.WriteHeader(http.StatusInternalServerError)

		return
	}

	w.Write([]byte("logged in to Common Fate successfully! You can close this window."))

	s.response <- data
}

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(20 * time.Minute)

	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)

	return state
}

func (s *Server) getUserData(code string) (Response, error) {
	// Use code to get token and get user info.
	cfg := s.oauthConfig()
	clio.Debugw("exchanging oauth code", "oauth.config", cfg)

	t, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return Response{}, fmt.Errorf("code exchange error: %s", err.Error())
	}

	IDToken, ok := t.Extra("id_token").(string)
	if !ok {
		return Response{}, errors.New("could not find id_token in authentication response")
	}

	// currently, our Cognito REST API authentication uses the ID Token rather than the Access Token.
	// for simplicity, we override the returned access token with the ID token,
	// as the oauth2 package appends the access token automatically to outgoing requests.
	t.AccessToken = IDToken

	res := Response{
		Token:   t,
		Context: s.context,
	}

	return res, nil
}
