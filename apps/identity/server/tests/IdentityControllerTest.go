import (
	"errors"
	"net/http"
)

// @Test(KeycloakTestSetup, {"partition":"tests"})
type IdentityControllerTest struct {
	behaviors *BehaviorContext
}

func (test *IdentityControllerTest) TestGetMeDescribesTheSignedInMember() {
	setup := KeycloakTestSetup{}
	realm := setup.Realm("example.com")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	request.Header.Set("Authorization", "Bearer "+setup.Token(realm, setup.Member()))
	me, err := (&IdentityController{}).GetMe(request)
	Assert.IsTrue(err == nil)
	Assert.Equals("member-1", me.MemberId)
	Assert.Equals("example.com", me.DomainId)
	Assert.Equals("Ana Example", me.DisplayName)
	Assert.Equals("ana@example.com", me.PrimaryAddress)
	Assert.Equals("Member", me.Roles[0])
}

func (test *IdentityControllerTest) TestGetMeRefusesAMissingToken() {
	KeycloakTestSetup{}.Realm("example.com")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	_, err := (&IdentityController{}).GetMe(request)
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}
