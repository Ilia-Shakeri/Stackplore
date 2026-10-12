package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/domain"
)

type authenticationBoundary interface {
	SetupStatus(context.Context) (auth.SetupStatus, error)
	CreatePreauth(context.Context, auth.PreauthPurpose, string) (auth.PreauthIssue, error)
	Setup(context.Context, auth.SetupInput) (auth.SessionIssue, error)
	Login(context.Context, auth.LoginInput) (auth.SessionIssue, error)
	Authenticate(context.Context, string) (auth.Authentication, error)
	AuthorizeUnsafe(context.Context, string, string) (auth.Authentication, error)
	AuthorizeRecent(context.Context, auth.RecentAuthorizationInput) (auth.Authentication, error)
	Logout(context.Context, auth.LogoutInput) error
	CreateRecentGrant(context.Context, auth.RecentAuthInput) (auth.RecentGrantIssue, error)
	ChangeOwnPassword(context.Context, auth.OwnPasswordChangeInput) (auth.SessionIssue, error)
	ListUsers(context.Context, auth.AuthenticatedReadInput, auth.UserListQuery) (auth.UserPage, error)
	GetUser(context.Context, auth.AuthenticatedReadInput, auth.UserID) (auth.User, error)
	CreateUser(context.Context, auth.UserCreateInput) (auth.User, error)
	UpdateUser(context.Context, auth.UserUpdateInput) (auth.User, error)
	DeleteUser(context.Context, auth.UserDeleteInput) error
	ResetUserPassword(context.Context, auth.UserPasswordResetInput) error
	RevokeUserSessions(context.Context, auth.UserSessionsRevokeInput) error
}

var _ authenticationBoundary = (*auth.Service)(nil)

type secretInput []byte

func (secret *secretInput) UnmarshalJSON(contents []byte) error {
	if secret == nil {
		return errJSONBodyInvalid
	}
	var value string
	if err := json.Unmarshal(contents, &value); err != nil {
		return errJSONBodyInvalid
	}
	*secret = append((*secret)[:0], value...)
	return nil
}

func (secret secretInput) String() string { return "[protected input]" }

func clearSecret(secret secretInput) { clear(secret) }

type setupStatusResponse struct {
	State                  string `json:"state"`
	BootstrapProofRequired bool   `json:"bootstrap_proof_required"`
}

type preauthRequest struct {
	Purpose auth.PreauthPurpose `json:"purpose"`
}

type preauthResponse struct {
	Proof     string `json:"preauth_proof"`
	ExpiresAt string `json:"expires_at"`
}

type setupRequest struct {
	LoginName      string      `json:"login_name"`
	DisplayName    string      `json:"display_name"`
	Password       secretInput `json:"password"`
	BootstrapProof secretInput `json:"bootstrap_proof,omitempty"`
}

type loginRequest struct {
	LoginName string      `json:"login_name"`
	Password  secretInput `json:"password"`
}

type recentGrantRequest struct {
	CurrentPassword secretInput            `json:"current_password"`
	Purpose         auth.RecentAuthPurpose `json:"purpose"`
}

type recentGrantResponse struct {
	Proof     string                 `json:"recent_auth_proof"`
	Purpose   auth.RecentAuthPurpose `json:"purpose"`
	ExpiresAt string                 `json:"expires_at"`
}

type ownPasswordRequest struct {
	NewPassword secretInput `json:"new_password"`
}

type createUserRequest struct {
	LoginName   string      `json:"login_name"`
	DisplayName string      `json:"display_name"`
	Role        auth.Role   `json:"role"`
	Password    secretInput `json:"password"`
}

type updateUserRequest struct {
	LoginName   *string    `json:"login_name,omitempty"`
	DisplayName *string    `json:"display_name,omitempty"`
	Role        *auth.Role `json:"role,omitempty"`
	Enabled     *bool      `json:"enabled,omitempty"`
}

type resetPasswordRequest struct {
	NewPassword secretInput `json:"new_password"`
}

type userResponse struct {
	ID          auth.UserID `json:"id"`
	LoginName   string      `json:"login_name"`
	DisplayName string      `json:"display_name"`
	Role        auth.Role   `json:"role"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
	Version     int64       `json:"version"`
}

type sessionResponse struct {
	User              userResponse `json:"user"`
	CSRFProof         string       `json:"csrf_proof"`
	IdleExpiresAt     string       `json:"idle_expires_at"`
	AbsoluteExpiresAt string       `json:"absolute_expires_at"`
}

type pageResponse struct {
	Limit      int    `json:"limit"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type userPageResponse struct {
	Items []userResponse `json:"items"`
	Page  pageResponse   `json:"page"`
}

func authUserResponse(user auth.User) userResponse {
	return userResponse{
		ID: user.ID, LoginName: user.LoginName, DisplayName: user.DisplayName,
		Role: user.Role, Enabled: user.Enabled, CreatedAt: authTimestamp(user.CreatedAt),
		UpdatedAt: authTimestamp(user.UpdatedAt), Version: user.Version,
	}
}

func authSessionResponse(contextValue auth.SessionContext) (sessionResponse, error) {
	proof, err := contextValue.CSRFProof.Encode()
	if err != nil {
		return sessionResponse{}, err
	}
	return sessionResponse{
		User: authUserResponse(contextValue.User), CSRFProof: proof,
		IdleExpiresAt:     authTimestamp(contextValue.IdleExpiresAt),
		AbsoluteExpiresAt: authTimestamp(contextValue.AbsoluteExpiresAt),
	}, nil
}

func authTimestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func requestIDForAuth(r *http.Request) domain.RequestID {
	return domain.RequestID(requestIDFromContext(r.Context()))
}

func authSourceKey(r *http.Request) (string, bool) {
	info := networkInfoFromContext(r.Context())
	if !info.clientVerified || !info.clientIP.IsValid() {
		return "", false
	}
	return info.clientIP.String(), true
}

func readAuthenticationMaterial(r *http.Request, csrfRequired bool, recentRequired bool) (string, string, string, problemKind, bool) {
	session, ok := readCookieValue(r, sessionCookieName)
	if !ok {
		return "", "", "", unauthenticatedProblem, false
	}
	csrf := ""
	if csrfRequired {
		csrf, ok = readSingleHeader(r, csrfHeaderName)
		if !ok {
			return "", "", "", forbiddenProblem, false
		}
	}
	recent := ""
	if recentRequired {
		recent, ok = readSingleHeader(r, recentAuthHeaderName)
		if !ok {
			return "", "", "", recentAuthProblem, false
		}
	}
	return session, csrf, recent, problemKind{}, true
}

func parseUserListQuery(r *http.Request) (auth.UserListQuery, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return auth.UserListQuery{}, errJSONBodyInvalid
	}
	for name, values := range query {
		if name != "cursor" && name != "limit" && name != "q" && name != "role" && name != "enabled" || len(values) != 1 {
			return auth.UserListQuery{}, errJSONBodyInvalid
		}
	}
	result := auth.UserListQuery{Limit: 50}
	result.Cursor = query.Get("cursor")
	result.Search = query.Get("q")
	if values, present := query["cursor"]; present && !validOpaqueCursor(values[0]) {
		return auth.UserListQuery{}, errJSONBodyInvalid
	}
	if !validUserSearchQuery(result.Search) {
		return auth.UserListQuery{}, errJSONBodyInvalid
	}
	if values, present := query["limit"]; present {
		value := values[0]
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 || strconv.Itoa(parsed) != value {
			return auth.UserListQuery{}, errJSONBodyInvalid
		}
		result.Limit = parsed
	}
	if values, present := query["role"]; present {
		value := values[0]
		role := auth.Role(value)
		if role.Validate() != nil {
			return auth.UserListQuery{}, errJSONBodyInvalid
		}
		result.Role = &role
	}
	if values, present := query["enabled"]; present {
		value := values[0]
		parsed, err := strconv.ParseBool(value)
		if err != nil || strconv.FormatBool(parsed) != value {
			return auth.UserListQuery{}, errJSONBodyInvalid
		}
		result.Enabled = &parsed
	}
	return result, nil
}

func validOpaqueCursor(value string) bool {
	if value == "" || len(value) > 1024 {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(decoded) > 0 && base64.RawURLEncoding.EncodeToString(decoded) == value
}

func validUserSearchQuery(value string) bool {
	if value == "" {
		return true
	}
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 || value != strings.TrimSpace(value) {
		return false
	}
	hasVisibleBase := false
	for _, character := range value {
		if unicode.IsControl(character) || unicode.In(character, unicode.Zl, unicode.Zp) ||
			unicode.In(character, unicode.Cf) && character != '\u200c' && character != '\u200d' {
			return false
		}
		if unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsPunct(character) || unicode.IsSymbol(character) {
			hasVisibleBase = true
		}
	}
	return hasVisibleBase
}

func parseExpectedVersion(r *http.Request) (int64, error) {
	values := r.Header.Values("If-Match")
	if len(values) != 1 || len(values[0]) < 3 || len(values[0]) > 18 || values[0][0] != '"' || values[0][len(values[0])-1] != '"' {
		return 0, errJSONBodyInvalid
	}
	text := values[0][1 : len(values[0])-1]
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil || parsed < 1 || strconv.FormatInt(parsed, 10) != text {
		return 0, errJSONBodyInvalid
	}
	return parsed, nil
}

func setUserEntityTag(w http.ResponseWriter, version int64) {
	w.Header().Set("ETag", `"`+strconv.FormatInt(version, 10)+`"`)
}

func writeBodyDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errJSONBodyTooLarge):
		writeProblem(w, r, payloadProblem)
	case errors.Is(err, errJSONMediaType):
		writeProblem(w, r, mediaTypeProblem)
	default:
		writeProblem(w, r, invalidRequestProblem)
	}
}
