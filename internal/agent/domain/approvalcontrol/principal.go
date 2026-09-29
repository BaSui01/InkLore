package approvalcontrol

import (
	authdomain "InkLore/internal/platform/auth"
	"strings"
)

const (
	PrincipalSourceApproval   = "approval"
	PrincipalSourceAuthCenter = "auth_center"
	PrincipalSourceAnonymous  = "anonymous"
)

type Principal struct {
	User        *authdomain.User
	Source      string
	Permissions []string
	Roles       []string
	UserID      int64
}

func PrincipalFromUser(user *authdomain.User) Principal {
	if user == nil {
		return AnonymousPrincipal()
	}
	authPrincipal := authdomain.PrincipalFromUser(user)
	return Principal{
		User:        user,
		UserID:      user.ID,
		Permissions: append([]string(nil), authPrincipal.Permissions...),
		Roles:       append([]string(nil), authPrincipal.Roles...),
		Source:      PrincipalSourceAuthCenter,
	}
}

func AnonymousPrincipal() Principal {
	return Principal{Source: PrincipalSourceAnonymous}
}

func NormalizeSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return PrincipalSourceAnonymous
	}
	return source
}
