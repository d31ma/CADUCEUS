import (
	"crypto/rand"
	"errors"
	"strings"
)

// @Test(GatewayTestSetup, {"partition":"tests"})
type GatewayControllerTest struct {
	behaviors *BehaviorContext
}

func (test *GatewayControllerTest) TestPostConnectAcceptsASocket() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	Assert.IsTrue((&GatewayController{}).PostConnect(GatewayConnect{ConnectionId: "c1", Kind: "socket"}) == nil, "a socket was refused")
}

func (test *GatewayControllerTest) TestPostConnectRefusesAnUnknownKind() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As((&GatewayController{}).PostConnect(GatewayConnect{ConnectionId: "c1", Kind: "pigeon"}), &invalid), "an unknown kind was accepted")
	Assert.Equals("kind", invalid.Field)
}

func (test *GatewayControllerTest) TestPostMessageAnswersAPing() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	reply, failure := (&GatewayController{}).PostMessage(GatewayMessage{ConnectionId: "c1", Body: `{"type":"ping"}`})
	Assert.IsTrue(failure == nil, "a ping failed")
	Assert.Equals("", reply.DomainId)
}

func (test *GatewayControllerTest) TestPostMessageRefusesAnUnknownType() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	_, failure := (&GatewayController{}).PostMessage(GatewayMessage{ConnectionId: "c1", Body: `{"type":"dance"}`})
	invalid := &InvalidRequest{}
	Assert.IsTrue(errors.As(failure, &invalid), "an unknown type was accepted")
}

func (test *GatewayControllerTest) TestPostMessageClosesAConnectionWithAForgedToken() {
	setup := GatewayTestSetup{}
	closes := make(chan string, 2)
	setup.Realm(setup.Gateway(closes))
	reply, failure := (&GatewayController{}).PostMessage(GatewayMessage{ConnectionId: "forged-1", Body: `{"type":"auth","token":"forged"}`})
	Assert.IsTrue(failure == nil, "a refused auth failed")
	Assert.Equals("", reply.DomainId)
	Assert.Equals("POST /@connections/forged-1 "+setup.Secret(), <-closes)
	Assert.Equals("DELETE /@connections/forged-1?code=4401 "+setup.Secret(), <-closes)
}

func (test *GatewayControllerTest) TestPostMessageAcceptsAMemberAndPostDisconnectRemovesIt() {
	setup := GatewayTestSetup{}
	closes := make(chan string, 2)
	realm := setup.Realm(setup.Gateway(closes))
	body := `{"type":"auth","token":"` + setup.Token(realm, []string{"mesail-member"}) + `"}`
	controller := &GatewayController{}
	reply, failure := controller.PostMessage(GatewayMessage{ConnectionId: "member-socket-1", Body: body})
	Assert.IsTrue(failure == nil, "a member was refused")
	Assert.Equals("example.com", reply.DomainId)
	Assert.Equals("POST /@connections/member-socket-1 "+setup.Secret(), <-closes)
	Assert.IsTrue(controller.PostDisconnect(GatewayDisconnect{ConnectionId: "member-socket-1", DomainId: "example.com"}) == nil, "disconnect failed")
}

func (test *GatewayControllerTest) TestPostMessageTypingBeforeAuthIsClosed() {
	setup := GatewayTestSetup{}
	closes := make(chan string, 2)
	setup.Realm(setup.Gateway(closes))
	_, failure := (&GatewayController{}).PostMessage(GatewayMessage{ConnectionId: "early-1", Body: `{"type":"typing","conversationId":"c1"}`})
	Assert.IsTrue(failure == nil, "typing before auth failed")
	Assert.Equals("POST /@connections/early-1 "+setup.Secret(), <-closes)
	Assert.Equals("DELETE /@connections/early-1?code=4401 "+setup.Secret(), <-closes)
}

func (test *GatewayControllerTest) TestPostMessageTypingFromANonMemberIsNotFound() {
	setup := GatewayTestSetup{}
	closes := make(chan string, 2)
	realm := setup.Realm(setup.Gateway(closes))
	body := `{"type":"auth","token":"` + setup.Token(realm, []string{"mesail-member"}) + `"}`
	controller := &GatewayController{}
	controller.PostMessage(GatewayMessage{ConnectionId: "typist-1", Body: body})
	_, failure := controller.PostMessage(GatewayMessage{ConnectionId: "typist-1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"lonely-` + strings.ToLower(rand.Text()) + `"}`})
	missing := &NotFound{}
	Assert.IsTrue(errors.As(failure, &missing), "a non-member was allowed to type")
}

func (test *GatewayControllerTest) TestPostDisconnectBeforeAuthDoesNothing() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	Assert.IsTrue((&GatewayController{}).PostDisconnect(GatewayDisconnect{ConnectionId: "c1"}) == nil, "disconnect failed")
}
