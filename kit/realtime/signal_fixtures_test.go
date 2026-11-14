package realtime

import (
	"crypto/rand"
	"errors"
	"strconv"
	"sync"
	"time"

	"mesail.dev/packages/oidc"
)

type memoryDirectory struct {
	mutex  sync.Mutex
	owners map[string]string
	down   bool
	asked  int
}

func (directory *memoryDirectory) Owners(domainId string, deviceIds []string) (map[string]string, error) {
	directory.mutex.Lock()
	defer directory.mutex.Unlock()
	directory.asked++
	if directory.down {
		return nil, Unavailable{Operation: "directory"}
	}
	found := map[string]string{}
	for _, deviceId := range deviceIds {
		if owner, known := directory.owners[deviceId]; known && domainId == "example.com" {
			found[deviceId] = owner
		}
	}
	return found, nil
}

type recordingNotify struct {
	mutex   sync.Mutex
	pushes  []CallPush
	domains []string
	down    bool
}

func (notify *recordingNotify) Send(domainId string, push CallPush) error {
	notify.mutex.Lock()
	defer notify.mutex.Unlock()
	if notify.down {
		return errors.New("notify is down")
	}
	notify.pushes = append(notify.pushes, push)
	notify.domains = append(notify.domains, domainId)
	return nil
}

type recordingPushLog struct {
	mutex  sync.Mutex
	failed []string
}

func (log *recordingPushLog) PushFailed(domainId string, kind string, failure error) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.failed = append(log.failed, domainId+" "+kind+" "+failure.Error())
}

type movingClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *movingClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *movingClock) move(by time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = clock.now.Add(by)
}

type signalFixture struct {
	gatewayFixture
	directory *memoryDirectory
	notify    *recordingNotify
	pushLog   *recordingPushLog
	clock     *movingClock
	key       oidc.SigningKey
}

func signalTestKey() oidc.SigningKey {
	key, err := oidc.SigningKey{}.Generated()
	if err != nil {
		panic(err)
	}
	return key
}

func newSignalFixture() signalFixture {
	base := newGatewayFixture()
	directory := &memoryDirectory{owners: map[string]string{"a1": "ana", "a2": "ana", "b1": "ben", "b2": "ben", "c1": "cara"}}
	notify := &recordingNotify{}
	pushLog := &recordingPushLog{}
	clock := &movingClock{now: connectionTestTime()}
	key := signalTestKey()
	storage := base.memory.storage()
	connections := base.gateway.Connections
	pairings := PairingBook{Storage: storage, Now: clock.Now}
	signer := CallSigner{Current: key, Now: clock.Now}
	calls := Calls{Book: CallBook{Storage: storage}, Seats: SeatBook{Storage: storage}, Starts: CallStartBook{Storage: storage, Now: clock.Now}, Signer: signer, Connections: connections, Pushes: base.pushes, Notify: notify, Log: pushLog, Now: clock.Now}
	base.gateway.Sessions.Devices = DeviceLinks{Directory: directory}
	base.gateway.Signalling = Signalling{
		Signals:  Signals{Connections: connections, Directory: directory, Pairings: pairings, Pushes: base.pushes},
		Pairings: Pairings{Book: pairings, Connections: connections, Pushes: base.pushes},
		Calls:    calls,
		Pushes:   base.pushes,
	}
	return signalFixture{gatewayFixture: base, directory: directory, notify: notify, pushLog: pushLog, clock: clock, key: key}
}

func (fixture signalFixture) device(connectionId string, token string, deviceId string) {
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: connectionId, Body: `{"type":"auth","token":"` + token + `","deviceId":"` + deviceId + `"}`})
	if err != nil {
		panic(err)
	}
}

func (fixture signalFixture) send(connectionId string, body string) error {
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: connectionId, DomainId: "example.com", Body: body})
	return err
}

func (fixture signalFixture) last(connectionId string) map[string]any {
	frames := fixture.pushes.framesOf(connectionId)
	if len(frames) == 0 {
		return map[string]any{}
	}
	return frames[len(frames)-1]
}

func (fixture signalFixture) callId(connectionId string) string {
	text, _ := fixture.last(connectionId)["callId"].(string)
	return text
}

func (fixture signalFixture) seatPeople(callId string, count int) {
	storage := fixture.memory.storage()
	for index := 0; index < count; index++ {
		person := "person" + strconv.Itoa(index)
		if err := (SeatBook{Storage: storage}).Add("example.com", SeatRecord{CallId: callId, DeviceId: person + "-device", PersonId: person}); err != nil {
			panic(err)
		}
	}
}

func randomBytes(size int) []byte {
	raw := make([]byte, size)
	rand.Read(raw)
	return raw
}
