package core

import "strings"

type Role string

const (
	RoleOwner   Role = "owner"
	RoleAdmin   Role = "admin"
	RoleWriter  Role = "writer"
	RoleReader  Role = "reader"
	RoleAuditor Role = "auditor"
)

type Action string

const (
	ActionStore     Action = "store"
	ActionRetrieve  Action = "retrieve"
	ActionAuditRead Action = "audit_read"
	ActionMetrics   Action = "metrics"
	ActionManageRoles Action = "manage_roles"
)

var rolePermissions = map[Role]map[Action]bool{
	RoleOwner: {
		ActionStore:     true,
		ActionRetrieve:  true,
		ActionAuditRead: true,
		ActionMetrics:   true,
		ActionManageRoles: true,
	},
	RoleAdmin: {
		ActionStore:     true,
		ActionRetrieve:  true,
		ActionAuditRead: true,
		ActionMetrics:   true,
		ActionManageRoles: true,
	},
	RoleWriter: {
		ActionStore:    true,
		ActionRetrieve: true,
		ActionMetrics:  true,
	},
	RoleReader: {
		ActionRetrieve: true,
		ActionMetrics:   true,
	},
	RoleAuditor: {
		ActionAuditRead: true,
		ActionMetrics:   true,
	},
}

func NormalizeRole(value string) Role {
	switch Role(strings.ToLower(strings.TrimSpace(value))) {
	case RoleOwner, RoleAdmin, RoleWriter, RoleReader, RoleAuditor:
		return Role(strings.ToLower(strings.TrimSpace(value)))
	default:
		return RoleWriter
	}
}

func Can(role string, action Action) bool {
	permissions, ok := rolePermissions[NormalizeRole(role)]
	if !ok {
		return false
	}
	return permissions[action]
}
