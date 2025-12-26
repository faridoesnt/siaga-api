package auth

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"siaga-api/api/constants"
	"siaga-api/api/contracts"
	"siaga-api/api/models/responses"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service endpoints require the application context.
type Service struct {
	app     *contracts.App
	repo    contracts.AuthRepository
}

func Init(app *contracts.App) contracts.AuthService {
	repo := initRepository(app)

	return &Service{
		app:     app,
		repo:    repo,
	}
}

// Login authenticates user and returns JWT + user object.
func (s *Service) Login(ctx context.Context, email, password string) (*contracts.AuthLoginResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return nil, responses.BadRequest(errors.New("email and password are required"))
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		// Do not reveal whether email exists.
		return nil, responses.UnAuthorized(errors.New("invalid email or password"))
	}
	if !user.Active {
		return nil, responses.Forbidden(errors.New("user is inactive"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, responses.UnAuthorized(errors.New("invalid email or password"))
	}

	ttlSeconds := parseInt(s.app.Config[constants.JWT_TTL], constants.DefaultJwtLifetime)
	expiration := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"user_id":  user.ID,
		"email":    user.Email,
		"name":     user.Name,
		"role":     user.Role,
		"exp":      expiration.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	secret := []byte(s.app.Config[constants.JWT_SECRET])
	signed, err := token.SignedString(secret)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	return &contracts.AuthLoginResult{
		AccessToken: signed,
		User:        user,
	}, nil
}

// LoginAdmin authenticates admin user and returns JWT + user object.
func (s *Service) LoginAdmin(ctx context.Context, email, password string) (*contracts.AuthLoginResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return nil, responses.BadRequest(errors.New("email and password are required"))
	}

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}
	if user == nil {
		return nil, responses.UnAuthorized(errors.New("invalid email or password"))
	}
	if user.Role != "ADMIN" {
		return nil, responses.Forbidden(errors.New("user is not admin"))
	}
	if !user.Active {
		return nil, responses.Forbidden(errors.New("user is inactive"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, responses.UnAuthorized(errors.New("invalid email or password"))
	}

	// Ensure admin has default permissions on first login.
	if err := s.ensureDefaultAdminPermissions(ctx, user.ID); err != nil {
		return nil, responses.InternalServerError(err)
	}

	ttlSeconds := parseInt(s.app.Config[constants.JWT_TTL], constants.DefaultJwtLifetime)
	expiration := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"user_id":  user.ID,
		"email":    user.Email,
		"name":     user.Name,
		"role":     user.Role,
		"exp":      expiration.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := []byte(s.app.Config[constants.JWT_SECRET])
	signed, err := token.SignedString(secret)
	if err != nil {
		return nil, responses.InternalServerError(err)
	}

	return &contracts.AuthLoginResult{
		AccessToken: signed,
		User:        user,
	}, nil
}

func parseInt(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	if v, err := strconv.Atoi(raw); err == nil && v > 0 {
		return v
	}
	return fallback
}

func (s *Service) ensureDefaultAdminPermissions(ctx context.Context, userID int64) error {
	var count int
	if err := s.app.Ds.ReaderDB.GetContext(ctx, &count, `
		SELECT COUNT(1) FROM user_permissions WHERE user_id = ?
	`, userID); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	// Assign all existing permissions to this admin by default.
	if _, err := s.app.Ds.WriterDB.ExecContext(ctx, `
		INSERT INTO user_permissions (user_id, permission_code)
		SELECT ?, code FROM permissions
	`, userID); err != nil {
		return err
	}
	return nil
}
