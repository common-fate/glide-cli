package tokenstore

import (
	"github.com/99designs/keyring"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type Storage struct {
	keyring cfKeyring
	Context string
}

// New creates a new token storage driver.
// The context is the authentication context to use.
// This is usually 'default' and in future can be
// expanded to allow CLI users to switch between
// separate Common Fate tenancies.
func New(context string) Storage {
	return Storage{
		keyring: cfKeyring{},
		Context: context,
	}
}

var (
	ErrNotFound = errors.New("auth token not found")
)

// Token returns the OAuth2.0 token.
// It meets the TokenSource interface in the oauth2 package.
func (s *Storage) Token() (*oauth2.Token, error) {
	var t oauth2.Token
	err := s.keyring.Retrieve(s.key(), &t)
	if err == keyring.ErrKeyNotFound {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, errors.Wrap(err, "keyring error")
	}

	return &t, nil
}

// Save the token
func (s *Storage) Save(token *oauth2.Token) error {
	return s.keyring.Store(s.key(), token)
}

// Clear the token
func (s *Storage) Clear() error {
	return s.keyring.Clear(s.key())
}

// key of the keyring item includes the context name in it.
func (s *Storage) key() string {
	return "authtoken_" + s.Context
}
