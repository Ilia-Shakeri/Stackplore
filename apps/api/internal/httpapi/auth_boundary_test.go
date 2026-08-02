package httpapi

import (
	"context"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/auth"
)

type testAuthentication struct {
	setupStatusFunc        func(context.Context) (auth.SetupStatus, error)
	createPreauthFunc      func(context.Context, auth.PreauthPurpose, string) (auth.PreauthIssue, error)
	setupFunc              func(context.Context, auth.SetupInput) (auth.SessionIssue, error)
	loginFunc              func(context.Context, auth.LoginInput) (auth.SessionIssue, error)
	authenticateFunc       func(context.Context, string) (auth.Authentication, error)
	logoutFunc             func(context.Context, auth.LogoutInput) error
	createRecentGrantFunc  func(context.Context, auth.RecentAuthInput) (auth.RecentGrantIssue, error)
	changeOwnPasswordFunc  func(context.Context, auth.OwnPasswordChangeInput) (auth.SessionIssue, error)
	listUsersFunc          func(context.Context, auth.AuthenticatedReadInput, auth.UserListQuery) (auth.UserPage, error)
	getUserFunc            func(context.Context, auth.AuthenticatedReadInput, auth.UserID) (auth.User, error)
	createUserFunc         func(context.Context, auth.UserCreateInput) (auth.User, error)
	updateUserFunc         func(context.Context, auth.UserUpdateInput) (auth.User, error)
	deleteUserFunc         func(context.Context, auth.UserDeleteInput) error
	resetUserPasswordFunc  func(context.Context, auth.UserPasswordResetInput) error
	revokeUserSessionsFunc func(context.Context, auth.UserSessionsRevokeInput) error
}

func (service *testAuthentication) SetupStatus(ctx context.Context) (auth.SetupStatus, error) {
	if service.setupStatusFunc != nil {
		return service.setupStatusFunc(ctx)
	}
	return auth.SetupStatus{}, nil
}

func (service *testAuthentication) CreatePreauth(ctx context.Context, purpose auth.PreauthPurpose, source string) (auth.PreauthIssue, error) {
	if service.createPreauthFunc != nil {
		return service.createPreauthFunc(ctx, purpose, source)
	}
	return auth.PreauthIssue{}, auth.ErrAuthUnavailable
}

func (service *testAuthentication) Setup(ctx context.Context, input auth.SetupInput) (auth.SessionIssue, error) {
	if service.setupFunc != nil {
		return service.setupFunc(ctx, input)
	}
	return auth.SessionIssue{}, auth.ErrAuthUnavailable
}

func (service *testAuthentication) Login(ctx context.Context, input auth.LoginInput) (auth.SessionIssue, error) {
	if service.loginFunc != nil {
		return service.loginFunc(ctx, input)
	}
	return auth.SessionIssue{}, auth.ErrLoginRejected
}

func (service *testAuthentication) Authenticate(ctx context.Context, token string) (auth.Authentication, error) {
	if service.authenticateFunc != nil {
		return service.authenticateFunc(ctx, token)
	}
	return auth.Authentication{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) Logout(ctx context.Context, input auth.LogoutInput) error {
	if service.logoutFunc != nil {
		return service.logoutFunc(ctx, input)
	}
	return auth.ErrUnauthenticated
}

func (service *testAuthentication) CreateRecentGrant(ctx context.Context, input auth.RecentAuthInput) (auth.RecentGrantIssue, error) {
	if service.createRecentGrantFunc != nil {
		return service.createRecentGrantFunc(ctx, input)
	}
	return auth.RecentGrantIssue{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) ChangeOwnPassword(ctx context.Context, input auth.OwnPasswordChangeInput) (auth.SessionIssue, error) {
	if service.changeOwnPasswordFunc != nil {
		return service.changeOwnPasswordFunc(ctx, input)
	}
	return auth.SessionIssue{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) ListUsers(ctx context.Context, input auth.AuthenticatedReadInput, query auth.UserListQuery) (auth.UserPage, error) {
	if service.listUsersFunc != nil {
		return service.listUsersFunc(ctx, input, query)
	}
	return auth.UserPage{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) GetUser(ctx context.Context, input auth.AuthenticatedReadInput, userID auth.UserID) (auth.User, error) {
	if service.getUserFunc != nil {
		return service.getUserFunc(ctx, input, userID)
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) CreateUser(ctx context.Context, input auth.UserCreateInput) (auth.User, error) {
	if service.createUserFunc != nil {
		return service.createUserFunc(ctx, input)
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) UpdateUser(ctx context.Context, input auth.UserUpdateInput) (auth.User, error) {
	if service.updateUserFunc != nil {
		return service.updateUserFunc(ctx, input)
	}
	return auth.User{}, auth.ErrUnauthenticated
}

func (service *testAuthentication) DeleteUser(ctx context.Context, input auth.UserDeleteInput) error {
	if service.deleteUserFunc != nil {
		return service.deleteUserFunc(ctx, input)
	}
	return auth.ErrUnauthenticated
}

func (service *testAuthentication) ResetUserPassword(ctx context.Context, input auth.UserPasswordResetInput) error {
	if service.resetUserPasswordFunc != nil {
		return service.resetUserPasswordFunc(ctx, input)
	}
	return auth.ErrUnauthenticated
}

func (service *testAuthentication) RevokeUserSessions(ctx context.Context, input auth.UserSessionsRevokeInput) error {
	if service.revokeUserSessionsFunc != nil {
		return service.revokeUserSessionsFunc(ctx, input)
	}
	return auth.ErrUnauthenticated
}
