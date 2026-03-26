// Package handlers contains the huma HTTP handler implementations.
package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"

	"math/big"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/tanjd/bookshelf/internal/middleware"
	"github.com/tanjd/bookshelf/internal/models"
	"github.com/tanjd/bookshelf/internal/repository"
	"github.com/tanjd/bookshelf/internal/services"
)

// validatePasswordComplexity checks that p meets minimum complexity requirements.
// Returns a human-readable error string, or "" if valid.
func validatePasswordComplexity(p string) string {
	if len(p) < 8 {
		return "password must be at least 8 characters"
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range p {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper {
		return "password must contain at least one uppercase letter"
	}
	if !hasLower {
		return "password must contain at least one lowercase letter"
	}
	if !hasDigit {
		return "password must contain at least one number"
	}
	return ""
}

// AuthHandler holds dependencies for authentication routes.
type AuthHandler struct {
	users     repository.UserRepository
	admin     repository.AdminRepository
	jwtSecret string
	email     *services.EmailService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(users repository.UserRepository, admin repository.AdminRepository, jwtSecret string, email *services.EmailService) *AuthHandler {
	return &AuthHandler{users: users, admin: admin, jwtSecret: jwtSecret, email: email}
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

type meBody struct {
	models.User
	GoogleBooksKeyConfigured bool `json:"google_books_key_configured"`
}

type meOutput struct{ Body meBody }

type updateMeInput struct {
	Body struct {
		Name              *string `json:"name,omitempty" doc:"New display name"`
		Phone             *string `json:"phone,omitempty" doc:"Contact phone number"`
		Email             *string `json:"email,omitempty" format:"email" doc:"New email address"`
		GoogleBooksAPIKey *string `json:"google_books_api_key,omitempty" doc:"Your Google Books API key. Set to empty string to remove."`
	}
}

type setupStatusOutput struct {
	Body struct {
		NeedsSetup bool `json:"needs_setup"`
	}
}

type setupInput struct {
	Body struct {
		Name     string `json:"name" required:"true" minLength:"1" doc:"Admin display name"`
		Email    string `json:"email" required:"true" format:"email" doc:"Admin email address"`
		Password string `json:"password" required:"true" minLength:"8" doc:"Admin password (min 8 chars)"`
	}
}

type sendOTPInput struct{}

type verifyOTPInput struct {
	Body struct {
		Code string `json:"code" required:"true" doc:"6-digit OTP code"`
	}
}

type changePasswordInput struct {
	Body struct {
		CurrentPassword string `json:"current_password" required:"true" minLength:"1" doc:"Current password"`
		NewPassword     string `json:"new_password" required:"true" minLength:"8" doc:"New password (min 8 chars, mixed case + digit)"`
		ConfirmPassword string `json:"confirm_password" required:"true" minLength:"1" doc:"Must match new_password"`
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

	huma.Register(api, huma.Operation{
		OperationID: "setup-status",
		Method:      "GET",
		Path:        "/auth/setup-status",
		Tags:        []string{"auth"},
		Summary:     "Check whether initial admin setup is required",
	}, h.setupStatus)

	huma.Register(api, huma.Operation{
		OperationID:   "setup",
		Method:        "POST",
		Path:          "/auth/setup",
		Tags:          []string{"auth"},
		Summary:       "Create the initial admin account (one-time, fails if admin already exists)",
		DefaultStatus: 201,
	}, h.setup)

	huma.Register(api, huma.Operation{
		OperationID: "send-otp",
		Method:      "POST",
		Path:        "/auth/send-otp",
		Tags:        []string{"auth"},
		Summary:     "Send a 6-digit OTP to the authenticated user's email for verification",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.sendOTP)

	huma.Register(api, huma.Operation{
		OperationID: "verify-otp",
		Method:      "POST",
		Path:        "/auth/verify-otp",
		Tags:        []string{"auth"},
		Summary:     "Verify the OTP and mark the user as verified",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.verifyOTP)

	huma.Register(api, huma.Operation{
		OperationID: "change-password",
		Method:      "POST",
		Path:        "/auth/me/password",
		Tags:        []string{"auth"},
		Summary:     "Change the authenticated user's password",
		Security:    []map[string][]string{{"bearer": {}}},
	}, h.changePassword)
}

// --- Handlers ---

func (h *AuthHandler) register(_ context.Context, input *registerInput) (*authOutput, error) {
	if val, _ := h.admin.GetSetting("allow_registration"); val == "false" {
		return nil, huma.Error403Forbidden("registration is currently disabled")
	}
	if input.Body.Name == "" || input.Body.Email == "" || input.Body.Password == "" {
		return nil, huma.Error400BadRequest("name, email and password are required")
	}
	if msg := validatePasswordComplexity(input.Body.Password); msg != "" {
		return nil, huma.Error400BadRequest(msg)
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
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) login(_ context.Context, input *loginInput) (*authOutput, error) {
	user, err := h.users.FindByEmail(input.Body.Email)
	if err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.Password)); err != nil {
		return nil, huma.Error401Unauthorized("invalid credentials")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: *user}}, nil
}

func (h *AuthHandler) me(ctx context.Context, _ *struct{}) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
}

func (h *AuthHandler) updateMe(ctx context.Context, input *updateMeInput) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if input.Body.Name != nil {
		user.Name = *input.Body.Name
	}
	if input.Body.Phone != nil {
		user.Phone = *input.Body.Phone
	}
	if input.Body.Email != nil && *input.Body.Email != user.Email {
		existing, findErr := h.users.FindByEmail(*input.Body.Email)
		if findErr == nil && existing.ID != user.ID {
			return nil, huma.Error400BadRequest("email already in use")
		}
		user.Email = *input.Body.Email
		// New email must be re-verified.
		user.Verified = false
		user.OTPCode = ""
		user.OTPExpiry = nil
	}

	if input.Body.GoogleBooksAPIKey != nil {
		if *input.Body.GoogleBooksAPIKey == "" {
			user.GoogleBooksAPIKey = ""
		} else {
			if err := validateGoogleBooksAPIKey(*input.Body.GoogleBooksAPIKey); err != nil {
				return nil, huma.Error422UnprocessableEntity("invalid Google Books API key")
			}
			encrypted, err := encryptField(*input.Body.GoogleBooksAPIKey, h.jwtSecret)
			if err != nil {
				return nil, huma.Error500InternalServerError("could not save API key")
			}
			user.GoogleBooksAPIKey = encrypted
		}
	}

	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
}

func (h *AuthHandler) setupStatus(_ context.Context, _ *struct{}) (*setupStatusOutput, error) {
	hasAdmin, err := h.users.HasAdmin()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check setup status")
	}
	out := &setupStatusOutput{}
	out.Body.NeedsSetup = !hasAdmin
	return out, nil
}

func (h *AuthHandler) setup(_ context.Context, input *setupInput) (*authOutput, error) {
	hasAdmin, err := h.users.HasAdmin()
	if err != nil {
		return nil, huma.Error500InternalServerError("could not check setup status")
	}
	if hasAdmin {
		return nil, huma.Error403Forbidden("setup already complete")
	}
	if msg := validatePasswordComplexity(input.Body.Password); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.Password), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user := models.User{
		Name:     input.Body.Name,
		Email:    input.Body.Email,
		Password: string(hash),
		Role:     "admin",
		Verified: true,
	}
	if err := h.users.Create(&user); err != nil {
		return nil, huma.Error400BadRequest("email already registered")
	}

	token, err := h.issueToken(user.ID, user.Role)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not issue token")
	}

	return &authOutput{Body: authResponse{Token: token, User: user}}, nil
}

func (h *AuthHandler) sendOTP(ctx context.Context, _ *sendOTPInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return nil, huma.Error500InternalServerError("could not generate OTP")
	}
	code := fmt.Sprintf("%06d", n.Int64())
	expiry := time.Now().Add(15 * time.Minute)

	user.OTPCode = code
	user.OTPExpiry = &expiry
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not save OTP")
	}

	html := fmt.Sprintf(
		"<p>Hi %s,</p><p>Your Bookshelf verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>",
		user.Name, code,
	)
	if err := h.email.SendEmail(user.Email, "Your Bookshelf verification code", html); err != nil {
		// Log but don't fail — user can retry.
		log.Warn().Err(err).Uint("user_id", userID).Msg("sendOTP: email delivery failed")
	}

	return nil, nil
}

func (h *AuthHandler) verifyOTP(ctx context.Context, input *verifyOTPInput) (*meOutput, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if user.OTPCode == "" || user.OTPExpiry == nil {
		return nil, huma.Error400BadRequest("no OTP has been sent")
	}
	if time.Now().After(*user.OTPExpiry) {
		return nil, huma.Error400BadRequest("OTP has expired")
	}
	// Use constant-time comparison to prevent timing attacks, and invalidate
	// the OTP on any wrong attempt to prevent brute-force enumeration.
	if subtle.ConstantTimeCompare([]byte(user.OTPCode), []byte(input.Body.Code)) != 1 {
		user.OTPCode = ""
		user.OTPExpiry = nil
		_ = h.users.Save(user) //nolint:errcheck
		return nil, huma.Error400BadRequest("invalid OTP code — please request a new one")
	}

	user.Verified = true
	user.OTPCode = ""
	user.OTPExpiry = nil
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update user")
	}

	return &meOutput{Body: meBody{User: *user, GoogleBooksKeyConfigured: user.GoogleBooksAPIKey != ""}}, nil
}

func (h *AuthHandler) changePassword(ctx context.Context, input *changePasswordInput) (*struct{}, error) {
	userID, err := middleware.GetRequiredUserID(ctx)
	if err != nil {
		return nil, huma.Error401Unauthorized("authentication required")
	}

	user, err := h.users.FindByID(userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, huma.Error404NotFound("user not found")
		}
		return nil, huma.Error500InternalServerError("could not fetch user")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Body.CurrentPassword)); err != nil {
		return nil, huma.Error400BadRequest("current password is incorrect")
	}

	if input.Body.NewPassword != input.Body.ConfirmPassword {
		return nil, huma.Error400BadRequest("new passwords do not match")
	}

	if msg := validatePasswordComplexity(input.Body.NewPassword); msg != "" {
		return nil, huma.Error400BadRequest(msg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Body.NewPassword), 12)
	if err != nil {
		return nil, huma.Error500InternalServerError("could not hash password")
	}

	user.Password = string(hash)
	if err := h.users.Save(user); err != nil {
		return nil, huma.Error500InternalServerError("could not update password")
	}

	log.Info().Uint("user_id", userID).Msg("password changed")
	return nil, nil
}

// issueToken creates a signed HS256 JWT for the given user with a 24-hour expiry.
func (h *AuthHandler) issueToken(userID uint, role string) (string, error) {
	claims := middleware.JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
