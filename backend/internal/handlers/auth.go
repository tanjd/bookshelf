// Package handlers contains the huma HTTP handler implementations.
package handlers

import (
	"context"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
)

// AuthHandler holds dependencies for authentication routes.
type AuthHandler struct {
	db        *gorm.DB
	jwtSecret string
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *gorm.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret}
}

// --- Input / Output types ---

type registerInput struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1" doc:"Display name"`
		Email    string `json:"email" required:"true" format:"email" doc:"Email address"`
		Password string `json:"password" required:"true" minLength:"8" doc:"Password (min 8 chars)"`
	}
}

type loginInput struct {
	Body struct {
		Email    string `json:"email" required:"true"`
		Password string `json:"password" required:"true"`
	}
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type authOutput struct{ Body authResponse }

type meOutput struct{ Body models.User }

type updateMeInput struct {
	Body struct {
		Name  *string `json:"name,omitempty" doc:"New display name"`
		Phone *string `json:"phone,omitempty" doc:"Contact phone number"`
	}
}

// --- Route registration ---

// RegisterRoutes registers all auth routes on the given huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "register",
		Method:        "POST",
		Path:          "/auth/register",
		Tags:          []string{"auth"},
		Summary:       "Register a new user",
		DefaultStatus: 201,
	}, h.register)

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      "POST",
		Path:        "/auth/login",
		Tags:        []string{"auth"},
		Summary:     "Log in and receive a JWT",
	}, h.login)

	huma.Register(api, huma.Operation{
		OperationID: "get-me",
		Method:      "GET",
		Path:        "/auth/me",
		Tags:        []string{"auth"},
		Summary:     "Get the authenticated user's profile",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.me)

	huma.Register(api, huma.Operation{
		OperationID: "update-me",
		Method:      "PATCH",
		Path:        "/auth/me",
		Tags:        []string{"auth"},
		Summary:     "Update the authenticated user's profile",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.updateMe)
}

// --- Handlers ---

func (h *AuthHandler) register(_ context.Context, input *registerInput) (*authOutput, error) {
	if input.Body.Name == "" || input.Body.Email == "" || input.Body.Password == "" {
		return nil, huma.Error400BadRequest("name, email and password are required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name:     input.Body.Name,
		Email:    input.Body.Email,
		Password: string(hash),
	}
	if result := h.db.Create(&user); result.Error != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) login(_ context.Context, input *loginInput) (*authOutput, error) {
	var user models.User
	if err := h.db.Where("email = ?", input.Body.Email).First(&user).Error; err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.Password)); err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	token, err := h.issueToken(user.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil, huma.Error404NotFound("user not found")
	}

	return &meOutput{Body: user}, nil
}

func (h *AuthHandler) updateMe(ctx context.Context, input *updateMeInput) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	var user models.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil, huma.Error404NotFound("user not found")
	}

	if input.Body.Name != nil {
		user.Name = *input.Body.Name
	}
	if input.Body.Phone != nil {
		user.Phone = *input.Body.Phone
	}

	if err := h.db.Save(&user).Error; err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: user}, nil
}

// issueToken creates a signed HS256 JWT for the given user ID with a 24-hour expiry.
func (h *AuthHandler) issueToken(userID uint) (string, error) {
	claims := middleware.JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
