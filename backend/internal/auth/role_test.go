package auth

import "testing"

func TestRoleHierarchy(t *testing.T) {
	if !RoleAdmin.AtLeast(RoleStaff) {
		t.Error("admin should satisfy a staff requirement")
	}
	if !RoleAdmin.AtLeast(RoleCustomer) || !RoleStaff.AtLeast(RoleCustomer) {
		t.Error("staff and admin should satisfy a customer requirement")
	}
	if RoleCustomer.AtLeast(RoleStaff) {
		t.Error("a customer must not satisfy a staff requirement")
	}
	if RoleStaff.AtLeast(RoleAdmin) {
		t.Error("staff must not satisfy an admin requirement")
	}
	if Role("nonsense").Valid() || Role("nonsense").AtLeast(RoleCustomer) {
		t.Error("an unknown role must not be valid or grant anything")
	}
	for _, r := range []Role{RoleCustomer, RoleStaff, RoleAdmin} {
		if !r.Valid() || !r.AtLeast(r) {
			t.Errorf("%s should be valid and satisfy itself", r)
		}
	}
}
