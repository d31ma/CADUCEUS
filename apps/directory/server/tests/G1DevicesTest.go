// tachyon: example secrets
import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type G1DevicesTest struct {
	behaviors *BehaviorContext
}

func (test *G1DevicesTest) TestTheFirstDeviceNeedsNoSignature() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	device, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(add.DeviceId, device.DeviceId)
	Assert.Equals(people["ana"].PersonId, device.PersonId)
	Assert.Equals("Ana's phone", device.Name)
	Assert.Equals(add.IdentityKey, device.IdentityKey)
	Assert.IsTrue(device.AddedAt != "", "want addedAt")
}

func (test *G1DevicesTest) TestALaterDeviceWithoutASignatureIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.first(realm, people)
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), ""))
	Assert.Equals(1056, test.number(err))
	Assert.IsTrue(test.forbidden(err), "want Forbidden")
}

func (test *G1DevicesTest) TestALaterDeviceSignedByAnExistingDeviceIsAdded() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	firstKeys, first := test.first(realm, people)
	add := test.signed(test.add(test.keys(), ""), first.DeviceId, firstKeys)
	device, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(add.DeviceId, device.DeviceId)
}

func (test *G1DevicesTest) TestASignatureOverOtherKeysIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	firstKeys, first := test.first(realm, people)
	add := test.signed(test.add(test.keys(), ""), first.DeviceId, firstKeys)
	add.AgreementKey = test.public(test.keys())
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1056, test.number(err))
}

func (test *G1DevicesTest) TestADeviceOfAnotherPersonCannotSign() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.first(realm, people)
	boKeys := test.keys()
	boDevice, boErr := test.controller().PostDevice(test.bearer(realm, people["bo"]), test.add(boKeys, ""))
	add := test.signed(test.add(test.keys(), ""), boDevice.DeviceId, boKeys)
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals("<nil>", fmt.Sprint(boErr))
	Assert.Equals(1056, test.number(err))
}

func (test *G1DevicesTest) TestTheBackupKeyFromTheFirstDeviceCanSignANewDevice() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	backup := test.keys()
	_, firstErr := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), test.public(backup)))
	add := test.signed(test.add(test.keys(), ""), "backup", backup)
	device, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals("<nil>", fmt.Sprint(firstErr))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(add.DeviceId, device.DeviceId)
}

func (test *G1DevicesTest) TestABackupSignatureWithoutABackupKeyIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.first(realm, people)
	backup := test.keys()
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.signed(test.add(test.keys(), ""), "backup", backup))
	Assert.Equals(1056, test.number(err))
}

func (test *G1DevicesTest) TestADeviceAddedWithTheBackupKeyCannotChangeIt() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	backup := test.keys()
	test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), test.public(backup)))
	add := test.signed(test.add(test.keys(), test.public(test.keys())), "backup", backup)
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1059, test.number(err))
}

func (test *G1DevicesTest) TestADeviceSignedByADeviceMayReplaceTheBackupKey() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	firstKeys, first := test.first(realm, people)
	newBackup := test.keys()
	_, addErr := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.signed(test.add(test.keys(), test.public(newBackup)), first.DeviceId, firstKeys))
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.signed(test.add(test.keys(), ""), "backup", newBackup))
	Assert.Equals("<nil>", fmt.Sprint(addErr))
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *G1DevicesTest) TestAnUnusedDeviceIdIsRequired() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, first := test.first(realm, people)
	add := test.add(test.keys(), "")
	add.DeviceId = first.DeviceId
	_, err := test.controller().PostDevice(test.bearer(realm, people["bo"]), add)
	Assert.Equals(1057, test.number(err))
}

func (test *G1DevicesTest) TestADeviceIdMustBeALowerCaseUuid() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	add.DeviceId = strings.ToUpper(add.DeviceId)
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1049, test.number(err))
}

func (test *G1DevicesTest) TestANameOfSixtyFiveCharactersIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	add.Name = strings.Repeat("n", 65)
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1050, test.number(err))
}

func (test *G1DevicesTest) TestAnEmptyNameIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	add.Name = "   "
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1050, test.number(err))
}

func (test *G1DevicesTest) TestAShortIdentityKeyIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	add.IdentityKey = base64.RawURLEncoding.EncodeToString([]byte("short"))
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1051, test.number(err))
}

func (test *G1DevicesTest) TestAPaddedAgreementKeyIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	add := test.add(test.keys(), "")
	add.AgreementKey += "="
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1052, test.number(err))
}

func (test *G1DevicesTest) TestABadBackupKeyIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), "not-a-key"))
	Assert.Equals(1053, test.number(err))
}

func (test *G1DevicesTest) TestAddingRefusesAForgedToken() {
	_, err := test.controller().PostDevice("Bearer forged", test.add(test.keys(), ""))
	Assert.IsTrue(test.unauthenticated(err), "want Unauthenticated")
}

func (test *G1DevicesTest) TestAddingRefusesACallerWhoIsNotAMemberOfTheTokensDomain() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, err := test.controller().PostDevice(test.bearerIn(realm, people["eve"], people["ana"].DomainIds[0]), test.add(test.keys(), ""))
	Assert.Equals(1004, test.number(err))
}

func (test *G1DevicesTest) TestTheListShowsTheNameOnlyOfTheCallersOwnDevices() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, anaDevice := test.first(realm, people)
	boDevice, _ := test.controller().PostDevice(test.bearer(realm, people["bo"]), test.add(test.keys(), ""))
	devices, err := test.controller().GetDevices(test.bearer(realm, people["ana"]), people["ana"].PersonId+", "+people["bo"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(2, len(devices))
	Assert.Equals("Ana's phone", test.byId(devices)[anaDevice.DeviceId].Name)
	Assert.Equals("", test.byId(devices)[boDevice.DeviceId].Name)
	Assert.Equals(people["bo"].PersonId, test.byId(devices)[boDevice.DeviceId].PersonId)
}

func (test *G1DevicesTest) TestTheListLeavesOutPeopleWhoAreNotMembersOfTheTokensDomain() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, eveErr := test.controller().PostDevice(test.bearerIn(realm, people["eve"], people["eve"].DomainIds[0]), test.add(test.keys(), ""))
	devices, err := test.controller().GetDevices(test.bearer(realm, people["ana"]), people["eve"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(eveErr))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(devices))
}

func (test *G1DevicesTest) TestTheListTakesAtMostFiftyPeople() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, err := test.controller().GetDevices(test.bearer(realm, people["ana"]), test.manyIds(51))
	Assert.Equals(1054, test.number(err))
}

func (test *G1DevicesTest) TestTheListNeedsAtLeastOnePerson() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, err := test.controller().GetDevices(test.bearer(realm, people["ana"]), " , ")
	Assert.Equals(1054, test.number(err))
}

func (test *G1DevicesTest) TestThePersonRemovesTheirOwnDevice() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, device := test.first(realm, people)
	err := test.controller().DeleteDevice(test.bearer(realm, people["ana"]), device.DeviceId)
	devices, _ := test.controller().GetDevices(test.bearer(realm, people["ana"]), people["ana"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(devices))
}

func (test *G1DevicesTest) TestAnotherPersonsDeviceIsNotFound() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, device := test.first(realm, people)
	err := test.controller().DeleteDevice(test.bearer(realm, people["bo"]), device.DeviceId)
	devices, _ := test.controller().GetDevices(test.bearer(realm, people["ana"]), people["ana"].PersonId)
	Assert.Equals(1058, test.number(err))
	Assert.Equals(1, len(devices))
}

func (test *G1DevicesTest) TestAMissingDeviceIsNotFound() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	err := test.controller().DeleteDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), "").DeviceId)
	Assert.Equals(1058, test.number(err))
}

func (test *G1DevicesTest) TestOwnersNamesOnlyDevicesOfMembersOfTheDomain() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, anaDevice := test.first(realm, people)
	eveDevice, _ := test.controller().PostDevice(test.bearerIn(realm, people["eve"], people["eve"].DomainIds[0]), test.add(test.keys(), ""))
	unknown := test.add(test.keys(), "").DeviceId
	query := DeviceOwnersQuery{DomainId: people["ana"].DomainIds[0], DeviceIds: []string{anaDevice.DeviceId, eveDevice.DeviceId, unknown}}
	owners, err := test.controller().PostOwners(DirectoryTestSetup{}.ServiceBearer(realm, people["ana"].DomainIds[0]), query)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(owners.Owners))
	Assert.Equals(people["ana"].PersonId, owners.Owners[anaDevice.DeviceId])
}

func (test *G1DevicesTest) TestOwnersRefusesAServiceTokenOfAnotherDomain() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, anaDevice := test.first(realm, people)
	query := DeviceOwnersQuery{DomainId: people["ana"].DomainIds[0], DeviceIds: []string{anaDevice.DeviceId}}
	owners, err := test.controller().PostOwners(DirectoryTestSetup{}.ServiceBearer(realm, people["eve"].DomainIds[0]), query)
	Assert.Equals(1003, test.number(err))
	Assert.Equals(0, len(owners.Owners))
}

func (test *G1DevicesTest) TestOwnersTakesAtMostTwoHundredDevices() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	query := DeviceOwnersQuery{DomainId: people["ana"].DomainIds[0], DeviceIds: strings.Split(test.manyIds(201), ",")}
	_, err := test.controller().PostOwners(DirectoryTestSetup{}.ServiceBearer(realm, people["ana"].DomainIds[0]), query)
	Assert.Equals(1055, test.number(err))
}

func (test *G1DevicesTest) TestOwnersLeavesOutADisabledPerson() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	_, anaDevice := test.first(realm, people)
	disabled := people["ana"]
	disabled.State = "disabled"
	test.people().Store(disabled)
	query := DeviceOwnersQuery{DomainId: people["ana"].DomainIds[0], DeviceIds: []string{anaDevice.DeviceId}}
	owners, err := test.controller().PostOwners(DirectoryTestSetup{}.ServiceBearer(realm, people["ana"].DomainIds[0]), query)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(owners.Owners))
}

func (test *G1DevicesTest) TestDisablingAPersonRemovesTheirDevices() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.first(realm, people)
	_, err := test.peopleController().PatchPerson(test.operator(realm), people["ana"].PersonId, PersonChange{State: "disabled"})
	devices, _ := test.deviceStorage().ByPerson(people["ana"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(devices))
}

func (test *G1DevicesTest) TestRenamingAPersonKeepsTheirDevices() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.first(realm, people)
	_, err := test.peopleController().PatchPerson(test.operator(realm), people["ana"].PersonId, PersonChange{DisplayName: "Ana Renamed"})
	devices, _ := test.deviceStorage().ByPerson(people["ana"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(1, len(devices))
}

func (test *G1DevicesTest) TestDeletingAPersonRemovesTheirDevicesAndBackupKey() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), test.public(test.keys())))
	err := test.peopleController().DeletePerson(test.operator(realm), people["ana"].PersonId)
	devices, _ := test.deviceStorage().ByPerson(people["ana"].PersonId)
	backupKey, _ := (&BackupKeyStorageService{documents: test.documents(), names: &CollectionNamesService{}}).ByPerson(people["ana"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, len(devices))
	Assert.Equals("", backupKey.BackupKey)
}

func (test *G1DevicesTest) TestASignedAddWithoutABackupKeyKeepsTheStoredOne() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	backup := test.keys()
	firstKeys := test.keys()
	first, firstErr := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(firstKeys, test.public(backup)))
	_, secondErr := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.signed(test.add(test.keys(), ""), first.DeviceId, firstKeys))
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.signed(test.add(test.keys(), ""), "backup", backup))
	Assert.Equals("<nil>", fmt.Sprint(firstErr))
	Assert.Equals("<nil>", fmt.Sprint(secondErr))
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *G1DevicesTest) TestDisablingAPersonKeepsTheirBackupKey() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	backup := test.keys()
	test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(test.keys(), test.public(backup)))
	_, err := test.peopleController().PatchPerson(test.operator(realm), people["ana"].PersonId, PersonChange{State: "disabled"})
	stored, _ := (&BackupKeyStorageService{documents: test.documents(), names: &CollectionNamesService{}}).ByPerson(people["ana"].PersonId)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(test.public(backup), stored.BackupKey)
}

func (test *G1DevicesTest) TestAShortSignatureIsRefused() {
	realm, people, closeWorld := test.world()
	defer closeWorld()
	firstKeys, first := test.first(realm, people)
	add := test.signed(test.add(test.keys(), ""), first.DeviceId, firstKeys)
	add.Signature = add.Signature[:40]
	_, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), add)
	Assert.Equals(1056, test.number(err))
}

func (test *G1DevicesTest) first(realm oidc.SigningRealm, people map[string]PersonDocument) (ed25519.PrivateKey, Device) {
	keys := test.keys()
	device, err := test.controller().PostDevice(test.bearer(realm, people["ana"]), test.add(keys, ""))
	Assert.Equals("<nil>", fmt.Sprint(err))
	return keys, device
}

func (test *G1DevicesTest) add(identity ed25519.PrivateKey, backupKey string) DeviceAdd {
	return DeviceAdd{DeviceId: test.uuid(), Name: "  Ana's phone ", IdentityKey: test.public(identity), AgreementKey: test.public(test.keys()), BackupKey: backupKey}
}

func (test *G1DevicesTest) signed(add DeviceAdd, signerId string, signer ed25519.PrivateKey) DeviceAdd {
	text := strings.Join([]string{"mesail-device-v1", add.DeviceId, add.IdentityKey, add.AgreementKey, add.BackupKey}, "\n")
	add.SignerDeviceId = signerId
	add.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(signer, []byte(text)))
	return add
}

func (test *G1DevicesTest) keys() ed25519.PrivateKey {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	Assert.Equals("<nil>", fmt.Sprint(err))
	return private
}

func (test *G1DevicesTest) public(private ed25519.PrivateKey) string {
	return base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
}

func (test *G1DevicesTest) uuid() string {
	random := make([]byte, 16)
	rand.Read(random)
	text := hex.EncodeToString(random)
	return text[0:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:32]
}

func (test *G1DevicesTest) manyIds(count int) string {
	ids := []string{}
	for index := 0; index < count; index++ {
		ids = append(ids, fmt.Sprintf("id-%d", index))
	}
	return strings.Join(ids, ",")
}

func (test *G1DevicesTest) byId(devices []Device) map[string]Device {
	found := map[string]Device{}
	for _, device := range devices {
		found[device.DeviceId] = device
	}
	return found
}

func (test *G1DevicesTest) number(err error) int {
	var numbered interface{ Number() int }
	errors.As(err, &numbered)
	return test.numberOf(numbered)
}

func (test *G1DevicesTest) numberOf(numbered interface{ Number() int }) int {
	if numbered == nil {
		return 0
	}
	return numbered.Number()
}

func (test *G1DevicesTest) forbidden(err error) bool {
	var refused *Forbidden
	return errors.As(err, &refused)
}

func (test *G1DevicesTest) unauthenticated(err error) bool {
	var refused *Unauthenticated
	return errors.As(err, &refused)
}

func (test *G1DevicesTest) controller() *DevicesController {
	callers := &CallerService{}
	admission := &AdmissionService{callers: callers, people: test.people(), tenants: &TenantService{domains: test.domains()}}
	return &DevicesController{devices: &DeviceRouteService{admission: admission, callers: callers, devices: test.deviceService()}}
}

func (test *G1DevicesTest) peopleController() *PeopleController {
	people := &PersonService{people: test.people(), access: &PersonAccessService{people: test.people(), accounts: test.accounts()}, accounts: test.accounts(), writer: test.writer(), domains: test.domains()}
	return &PeopleController{people: &PersonRouteService{callers: &CallerService{}, people: people, devices: test.deviceService()}}
}

func (test *G1DevicesTest) deviceService() *DeviceService {
	names := &CollectionNamesService{}
	return &DeviceService{devices: test.deviceStorage(), backupKeys: &BackupKeyStorageService{documents: test.documents(), names: names}, people: test.people()}
}

func (test *G1DevicesTest) deviceStorage() *DeviceStorageService {
	return &DeviceStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *G1DevicesTest) accounts() *AccountAdminService {
	return &AccountAdminService{settings: &SettingsService{}}
}

func (test *G1DevicesTest) writer() *MemberWriterService {
	names := &CollectionNamesService{}
	members := &MemberStorageService{documents: test.documents(), names: names}
	store := &OutboxStoreService{domains: test.domains(), members: members, names: names}
	relay := &OutboxRelayService{store: store, settings: &SettingsService{}, domains: test.domains()}
	return &MemberWriterService{members: members, addresses: &AddressStorageService{documents: test.documents(), names: names}, events: &EventService{settings: &SettingsService{}}, relay: relay}
}

func (test *G1DevicesTest) people() *PeopleStorageService {
	return &PeopleStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *G1DevicesTest) domains() *DomainStorageService {
	return &DomainStorageService{documents: test.documents(), names: &CollectionNamesService{}}
}

func (test *G1DevicesTest) documents() *DocumentsService {
	return &DocumentsService{dataSource: DataSource{}}
}

func (test *G1DevicesTest) world() (oidc.SigningRealm, map[string]PersonDocument, func()) {
	realm := DirectoryTestSetup{}.SignedRealm()
	server := httptest.NewServer(http.HandlerFunc(test.keycloakAnswer))
	os.Setenv("YON_APPLICATION_JSON", DirectoryTestSetup{}.ApplicationJson(realm, server.URL))
	domainId, otherDomainId := test.hosted(), test.hosted()
	people := map[string]PersonDocument{"ana": test.member(domainId), "bo": test.member(domainId), "eve": test.member(otherDomainId)}
	return realm, people, func() { test.forget(people, domainId, otherDomainId); server.Close() }
}

func (test *G1DevicesTest) bearer(realm oidc.SigningRealm, person PersonDocument) string {
	return test.bearerIn(realm, person, person.DomainIds[0])
}

func (test *G1DevicesTest) bearerIn(realm oidc.SigningRealm, person PersonDocument, domainId string) string {
	return DirectoryTestSetup{}.PersonBearer(realm, person.PersonId, domainId, "mesail-member")
}

func (test *G1DevicesTest) operator(realm oidc.SigningRealm) string {
	return DirectoryTestSetup{}.PersonBearer(realm, "operator-"+test.unique(), "*", "mesail-operator")
}

func (test *G1DevicesTest) keycloakAnswer(writer http.ResponseWriter, request *http.Request) {
	switch {
	case strings.HasSuffix(request.URL.Path, "/protocol/openid-connect/token"):
		writer.Write([]byte(`{"access_token":"example-token"}`))
	case strings.HasSuffix(request.URL.Path, "/role-mappings/realm/composite"):
		writer.Write([]byte(`[{"name":"mesail-member"}]`))
	case request.Method == http.MethodGet:
		writer.Write([]byte(`{"id":"user","enabled":true,"attributes":{}}`))
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (test *G1DevicesTest) hosted() string {
	domainId := "g1-" + test.unique() + ".test"
	collections := &TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}
	Assert.Equals("<nil>", fmt.Sprint(collections.Prepare(domainId)))
	Assert.Equals("<nil>", fmt.Sprint(test.domains().Insert(DomainDocument{DomainId: domainId, Name: domainId, State: "active"})))
	return domainId
}

func (test *G1DevicesTest) member(domainId string) PersonDocument {
	person := PersonDocument{PersonId: "p-" + test.unique(), Username: "u" + test.unique(), DisplayName: "Person", State: "active"}
	person = PersonRules{}.With(person, domainId)
	_, err := test.writer().Join(person, domainId, false)
	saved, storeErr := test.people().Store(person)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint(storeErr))
	return saved
}

func (test *G1DevicesTest) forget(people map[string]PersonDocument, domainIds ...string) {
	service := test.deviceService()
	for _, person := range people {
		service.Forget(person.PersonId)
		test.documents().Delete((&CollectionNamesService{}).People(), person.Id)
	}
	for _, domainId := range domainIds {
		domain, _, _ := test.domains().Find(domainId)
		test.domains().Delete(domain)
		(&TenantCollectionsService{documents: test.documents(), names: &CollectionNamesService{}}).Drop(domainId, &WriteBudget{Left: 2, Deadline: time.Now().Add(time.Minute)})
	}
}

func (test *G1DevicesTest) unique() string {
	random := make([]byte, 6)
	rand.Read(random)
	return hex.EncodeToString(random)
}
