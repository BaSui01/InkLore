package authdomain

import "errors"

// PolicyDeniedError carries the policy decision that rejected a privileged action.
type PolicyDeniedError struct {
	Decision Decision
}

func (e *PolicyDeniedError) Error() string {
	reason := e.Decision.Reason
	if reason == "" {
		reason = "denied"
	}
	return "权限不足: " + reason
}

func NewPolicyDeniedError(decision Decision) error {
	return &PolicyDeniedError{Decision: decision}
}

func DenyUnlessAllowed(decision Decision) error {
	if decision.Allow {
		return nil
	}
	return NewPolicyDeniedError(decision)
}

// IsPolicyDenied reports whether an error came from PolicyService authorization.
func IsPolicyDenied(err error) bool {
	var denied *PolicyDeniedError
	return errors.As(err, &denied)
}

// ErrSSOUserConflict SSO 身份与本地已有账号产生冲突（例如用户名与邮箱分别对应不同的本地用户）
var ErrSSOUserConflict = errors.New("SSO 用户资料匹配到多个本地用户")
