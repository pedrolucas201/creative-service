package auth

import (
	"context"
	"errors"
	"strings"

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

type UserManager interface {
	CreateOrUpdateUserPassword(ctx context.Context, email, password string) (uid string, created bool, err error)
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

func (v *FirebaseVerifier) CreateOrUpdateUserPassword(
	ctx context.Context,
	email,
	password string,
) (uid string, created bool, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return "", false, errors.New("email is required")
	}
	if password == "" {
		return "", false, errors.New("password is required")
	}

	user, err := v.client.CreateUser(ctx, (&fbauth.UserToCreate{}).
		Email(email).
		Password(password),
	)
	if err == nil {
		return user.UID, true, nil
	}

	if !fbauth.IsEmailAlreadyExists(err) {
		return "", false, err
	}

	user, err = v.client.GetUserByEmail(ctx, email)
	if err != nil {
		return "", false, err
	}

	_, err = v.client.UpdateUser(ctx, user.UID, (&fbauth.UserToUpdate{}).
		Password(password),
	)
	if err != nil {
		return "", false, err
	}

	return user.UID, false, nil
}
