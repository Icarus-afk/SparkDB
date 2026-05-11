package rbac

import "testing"

func TestHasPermission(t *testing.T) {
	tests := []struct {
		role       Role
		perm       Permission
		expect     bool
	}{
		{RoleAdmin, PermQuery, true},
		{RoleAdmin, PermWrite, true},
		{RoleAdmin, PermCreate, true},
		{RoleAdmin, PermAlter, true},
		{RoleAdmin, PermDrop, true},
		{RoleAdmin, PermDelete, true},
		{RoleAdmin, PermCreateUser, true},
		{RoleAdmin, PermBackup, true},
		{RoleAdmin, PermRestore, true},
		{RoleAdmin, PermAuditLog, true},
		{RoleDeveloper, PermQuery, true},
		{RoleDeveloper, PermWrite, true},
		{RoleDeveloper, PermCreate, true},
		{RoleDeveloper, PermAlter, true},
		{RoleDeveloper, PermDelete, true},
		{RoleDeveloper, PermDrop, false},
		{RoleDeveloper, PermCreateUser, false},
		{RoleDeveloper, PermBackup, false},
		{RoleDeveloper, PermRestore, false},
		{RoleDeveloper, PermAuditLog, false},
		{RoleReadOnly, PermQuery, true},
		{RoleReadOnly, PermWrite, false},
		{RoleReadOnly, PermCreate, false},
		{RoleReadOnly, PermDrop, false},
		{RoleAuditor, PermAuditLog, true},
		{RoleAuditor, PermQuery, false},
		{RoleAuditor, PermWrite, false},
		{RoleAuditor, PermCreateUser, false},
	}

	for _, tt := range tests {
		got := HasPermission(tt.role, tt.perm)
		if got != tt.expect {
			t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.role, tt.perm, got, tt.expect)
		}
	}
}

func TestParseRole(t *testing.T) {
	valid := []string{"admin", "developer", "readonly", "auditor"}
	for _, r := range valid {
		role, ok := ParseRole(r)
		if !ok {
			t.Errorf("ParseRole(%q) should be valid", r)
		}
		if string(role) != r {
			t.Errorf("ParseRole(%q) = %q, want %q", r, role, r)
		}
	}

	if _, ok := ParseRole("superadmin"); ok {
		t.Error("ParseRole('superadmin') should be invalid")
	}
	if _, ok := ParseRole(""); ok {
		t.Error("ParseRole('') should be invalid")
	}
}

func TestValidRoles(t *testing.T) {
	roles := ValidRoles()
	if len(roles) != 4 {
		t.Errorf("ValidRoles() returned %d roles, want 4", len(roles))
	}
	seen := make(map[Role]bool)
	for _, r := range roles {
		if seen[r] {
			t.Errorf("duplicate role: %q", r)
		}
		seen[r] = true
	}
}
