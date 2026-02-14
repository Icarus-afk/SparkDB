package rbac

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDeveloper Role = "developer"
	RoleReadOnly  Role = "readonly"
	RoleAuditor   Role = "auditor"
)

type Permission string

const (
	PermQuery     Permission = "query"
	PermWrite     Permission = "write"
	PermCreate    Permission = "create"
	PermAlter     Permission = "alter"
	PermDrop      Permission = "drop"
	PermDelete    Permission = "delete"
	PermCreateUser Permission = "create_user"
	PermBackup    Permission = "backup"
	PermRestore   Permission = "restore"
	PermAuditLog  Permission = "audit_log"
)

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermQuery, PermWrite, PermCreate, PermAlter, PermDrop, PermDelete,
		PermCreateUser, PermBackup, PermRestore, PermAuditLog,
	},
	RoleDeveloper: {
		PermQuery, PermWrite, PermCreate, PermAlter, PermDelete,
	},
	RoleReadOnly: {
		PermQuery,
	},
	RoleAuditor: {
		PermAuditLog,
	},
}

func HasPermission(role Role, perm Permission) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

func ParseRole(s string) (Role, bool) {
	switch Role(s) {
	case RoleAdmin, RoleDeveloper, RoleReadOnly, RoleAuditor:
		return Role(s), true
	default:
		return "", false
	}

}

func ValidRoles() []Role {
	return []Role{RoleAdmin, RoleDeveloper, RoleReadOnly, RoleAuditor}
}
