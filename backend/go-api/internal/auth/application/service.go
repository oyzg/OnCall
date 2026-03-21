package application

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/oyzg/OnCall/backend/go-api/internal/auth/domain"
	"github.com/oyzg/OnCall/backend/go-api/pkg/config"
	"golang.org/x/crypto/bcrypt"
)

var errInvalidCredentials = errors.New("invalid credentials")

type LoginInput struct {
	Username string
	Password string
}

type LoginResult struct {
	AccessToken domain.AccessToken `json:"access_token"`
	User        domain.User        `json:"user"`
}

type Service struct {
	secret         []byte
	tokenExpiresIn time.Duration
	repo           Repository
	users          map[string]demoUser
}

type demoUser struct {
	User     domain.User
	Password string
}

type claims struct {
	UserID      string   `json:"uid"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
	jwt.RegisteredClaims
}

func NewService(cfg config.AuthConfig) *Service {
	return &Service{
		secret:         []byte(cfg.JWTSecret),
		tokenExpiresIn: cfg.TokenExpiresIn,
		users: map[string]demoUser{
			"admin": {
				User: domain.User{
					ID:          "user-admin",
					Username:    "admin",
					DisplayName: "Platform Admin",
					Roles:       []string{"admin"},
				},
				Password: "OnCallAdmin2026!",
			},
			"ops": {
				User: domain.User{
					ID:          "user-ops",
					Username:    "ops",
					DisplayName: "OnCall Operator",
					Roles:       []string{"ops"},
				},
				Password: "OnCallOps2026!",
			},
		},
	}
}

func NewServiceWithRepository(cfg config.AuthConfig, repo Repository) *Service {
	return &Service{
		secret:         []byte(cfg.JWTSecret),
		tokenExpiresIn: cfg.TokenExpiresIn,
		repo:           repo,
	}
}

func (s *Service) EnsureSeeded() error {
	if s.repo == nil {
		return nil
	}

	for username, credential := range demoCredentials() {
		if _, ok, err := s.repo.FindByUsername(nil, username); err != nil {
			return err
		} else if ok {
			continue
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(credential.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		record := StoredUser{
			User:         credential.User,
			Status:       "active",
			PasswordHash: string(passwordHash),
		}
		if err := s.repo.SaveUser(nil, record); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) Login(input LoginInput) (LoginResult, error) {
	if s.repo != nil {
		return s.loginWithRepository(input)
	}

	credential, ok := s.users[input.Username]
	if !ok || credential.Password != input.Password {
		return LoginResult{}, errInvalidCredentials
	}

	now := time.Now()
	expiresAt := now.Add(s.tokenExpiresIn)
	tokenClaims := claims{
		UserID:      credential.User.ID,
		Username:    credential.User.Username,
		DisplayName: credential.User.DisplayName,
		Roles:       credential.User.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   credential.User.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken: domain.AccessToken{
			Token:     signedToken,
			ExpiresIn: int64(s.tokenExpiresIn.Seconds()),
		},
		User: credential.User,
	}, nil
}

func (s *Service) loginWithRepository(input LoginInput) (LoginResult, error) {
	stored, ok, err := s.repo.FindByUsername(nil, strings.TrimSpace(input.Username))
	if err != nil {
		return LoginResult{}, err
	}
	if !ok || !isActiveStatus(stored.Status) {
		return LoginResult{}, errInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte(input.Password)); err != nil {
		return LoginResult{}, errInvalidCredentials
	}

	user := domain.User{
		ID:          stored.User.ID,
		Username:    stored.User.Username,
		DisplayName: stored.User.DisplayName,
		Roles:       append([]string(nil), stored.User.Roles...),
	}
	return s.issueToken(user)
}

func (s *Service) issueToken(user domain.User) (LoginResult, error) {
	now := time.Now()
	expiresAt := now.Add(s.tokenExpiresIn)
	tokenClaims := claims{
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Roles:       user.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	signedToken, err := token.SignedString(s.secret)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken: domain.AccessToken{
			Token:     signedToken,
			ExpiresIn: int64(s.tokenExpiresIn.Seconds()),
		},
		User: user,
	}, nil
}

func (s *Service) ParseUser(rawToken string) (domain.User, error) {
	token, err := jwt.ParseWithClaims(rawToken, &claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errInvalidCredentials
		}
		return s.secret, nil
	})
	if err != nil {
		return domain.User{}, errInvalidCredentials
	}

	tokenClaims, ok := token.Claims.(*claims)
	if !ok || !token.Valid {
		return domain.User{}, errInvalidCredentials
	}

	return domain.User{
		ID:          tokenClaims.UserID,
		Username:    tokenClaims.Username,
		DisplayName: tokenClaims.DisplayName,
		Roles:       tokenClaims.Roles,
	}, nil
}

func IsInvalidCredentials(err error) bool {
	return errors.Is(err, errInvalidCredentials)
}

func demoCredentials() map[string]demoUser {
	return map[string]demoUser{
		"admin": {
			User: domain.User{
				ID:          "user-admin",
				Username:    "admin",
				DisplayName: "Platform Admin",
				Roles:       []string{"admin"},
			},
			Password: "OnCallAdmin2026!",
		},
		"ops": {
			User: domain.User{
				ID:          "user-ops",
				Username:    "ops",
				DisplayName: "OnCall Operator",
				Roles:       []string{"ops"},
			},
			Password: "OnCallOps2026!",
		},
	}
}

func isActiveStatus(status string) bool {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "", "active", "enabled":
		return true
	default:
		return false
	}
}
