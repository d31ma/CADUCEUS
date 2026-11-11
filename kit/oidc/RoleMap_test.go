package oidc

import (
	"reflect"
	"testing"
)

func TestMesailRolesKeepsKnownRolesInOrder(t *testing.T) {
	claims := Claims{RealmRoles: []string{"mesail-admin", "offline_access", "mesail-member"}}
	got := RoleMap{}.MesailRoles(claims)
	if !reflect.DeepEqual(got, []string{"DomainAdmin", "Member"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMesailRolesIsEmptyWithoutRealmRoles(t *testing.T) {
	if got := (RoleMap{}).MesailRoles(Claims{}); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestMesailRolesMapsTheServiceRole(t *testing.T) {
	got := RoleMap{}.MesailRoles(Claims{RealmRoles: []string{"mesail-service"}})
	if !reflect.DeepEqual(got, []string{"Service"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMesailRolesMapsTheOperatorRole(t *testing.T) {
	got := RoleMap{}.MesailRoles(Claims{RealmRoles: []string{"mesail-operator"}})
	if !reflect.DeepEqual(got, []string{"Operator"}) {
		t.Fatalf("got %v", got)
	}
}
