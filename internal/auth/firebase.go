package auth

import (
	"context"
	"errors"

	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
)

type Identity struct {
	UID   string
	Email string
}

type Verifier interface {
	VerifyIDToken(ctx context.Context, token string) (*Identity, error)
}

type FirebaseVerifier struct {
	client *fbauth.Client
}

func NewFirebaseVerifier(ctx context.Context, projectID string) (*FirebaseVerifier, error) {
	if projectID == "" {
		return nil, errors.New("firebase project id is required")
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}

	return &FirebaseVerifier{client: client}, nil
}

func (v *FirebaseVerifier) VerifyIDToken(ctx context.Context, token string) (*Identity, error) {
	decoded, err := v.client.VerifyIDToken(ctx, token)
	if err != nil {
		return nil, err
	}

	email, _ := decoded.Claims["email"].(string)

	return &Identity{
		UID:   decoded.UID,
		Email: email,
	}, nil
}
