package approvalcontrol

import (
	authdomain "InkLore/internal/platform/auth"
	"strings"
)

const (
	PermAgentApprovalApprove     = authdomain.PermAgentApprovalApprove
	PermGenerationExecute        = authdomain.PermGenerationExecute
	AuthCenterPermissionWildcard = authdomain.AuthCenterPermissionWildcard
)

func CanApproveAgentTool(principal Principal) bool {
	if principal.UserID <= 0 {
		return false
	}
	return hasPermission(principal, PermAgentApprovalApprove)
}

func hasPermission(principal Principal, permission string) bool {
	if strings.TrimSpace(permission) == "" {
		return false
	}
	alts := authdomain.AuthCenterPermissionAlternatives(permission)
	for _, granted := range principal.Permissions {
		if granted == AuthCenterPermissionWildcard {
			return true
		}
		for _, alt := range alts {
			if granted == alt {
				return true
			}
		}
	}
	return false
}
