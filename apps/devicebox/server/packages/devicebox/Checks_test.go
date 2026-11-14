package devicebox

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func validPut() PutRequest {
	return PutRequest{EnvelopeId: "0f8fad5b-d9cb-469f-a165-70867728950e", SenderDeviceId: "a1", ToDeviceIds: []string{"b1"}, Kind: "message", Ciphertext: "aGVsbG8"}
}

func TestPutCheckRefusesEachBadFieldWithItsNumber(t *testing.T) {
	many := []string{}
	for index := 0; index < 201; index++ {
		many = append(many, "d"+strings.Repeat("x", index%5)+string(rune('a'+index%26))+strings.Repeat("y", index/26))
	}
	cases := []struct {
		change func(*PutRequest)
		number int
		field  string
	}{
		{func(r *PutRequest) { r.EnvelopeId = "not-a-uuid" }, 6003, "envelopeId"},
		{func(r *PutRequest) { r.SenderDeviceId = "" }, 6004, "senderDeviceId"},
		{func(r *PutRequest) { r.SenderDeviceId = "a/1" }, 6004, "senderDeviceId"},
		{func(r *PutRequest) { r.ToDeviceIds = nil }, 6005, "toDeviceIds"},
		{func(r *PutRequest) { r.ToDeviceIds = many }, 6005, "toDeviceIds"},
		{func(r *PutRequest) { r.ToDeviceIds = []string{"b1", "b1"} }, 6006, "toDeviceIds"},
		{func(r *PutRequest) { r.ToDeviceIds = []string{strings.Repeat("b", 129)} }, 6006, "toDeviceIds"},
		{func(r *PutRequest) { r.Kind = "undelivered" }, 6007, "kind"},
		{func(r *PutRequest) { r.Ciphertext = "" }, 6008, "ciphertext"},
		{func(r *PutRequest) { r.Ciphertext = "not base64!" }, 6008, "ciphertext"},
	}
	for _, testCase := range cases {
		request := validPut()
		testCase.change(&request)
		_, err := PutCheck{}.Checked(request)
		invalid := &InvalidRequest{}
		if !errors.As(err, &invalid) || invalid.Number() != testCase.number || invalid.Field != testCase.field || invalid.Error() == "" {
			t.Fatalf("%+v got %v", request, err)
		}
	}
}

func TestPutCheckAcceptsTwoHundredDevicesAnd256KiBButNotOneByteMore(t *testing.T) {
	request := validPut()
	request.ToDeviceIds = []string{}
	for index := 0; index < 200; index++ {
		request.ToDeviceIds = append(request.ToDeviceIds, "dev-"+strings.Repeat("z", index/26)+string(rune('a'+index%26)))
	}
	request.Ciphertext = base64.URLEncoding.EncodeToString(make([]byte, 256*1024))
	put, err := PutCheck{}.Checked(request)
	if err != nil || len(put.ToDeviceIds) != 200 {
		t.Fatalf("got %v", err)
	}
	request.Ciphertext = base64.RawURLEncoding.EncodeToString(make([]byte, 256*1024+1))
	_, err = PutCheck{}.Checked(request)
	tooLarge := &TooLarge{}
	if !errors.As(err, &tooLarge) || tooLarge.Number() != 6009 || tooLarge.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestPageCheckReadsTheCursorAndTheSize(t *testing.T) {
	after, err := PageCheck{}.After("")
	size, sizeError := PageCheck{}.Limit("")
	if after != 0 || err != nil || size != 50 || sizeError != nil {
		t.Fatalf("got %d %v %d %v", after, err, size, sizeError)
	}
	after, _ = PageCheck{}.After("1763110800000000")
	size, _ = PageCheck{}.Limit("100")
	if after != 1763110800000000 || size != 100 {
		t.Fatalf("got %d %d", after, size)
	}
	for _, text := range []string{"-1", "x", "1.5"} {
		if _, err := (PageCheck{}).After(text); numberOf(err) != 6015 {
			t.Fatalf("%q got %v", text, err)
		}
	}
	for _, text := range []string{"0", "101", "ten"} {
		if _, err := (PageCheck{}).Limit(text); numberOf(err) != 6016 {
			t.Fatalf("%q got %v", text, err)
		}
	}
}

func TestAcknowledgementCheckTakesOneToTenUuids(t *testing.T) {
	ids := []string{"0F8FAD5B-D9CB-469F-A165-70867728950E", "0f8fad5b-d9cb-469f-a165-70867728950e"}
	checked, err := AcknowledgementCheck{}.Checked(ids)
	if err != nil || len(checked) != 1 || checked[0] != "0f8fad5b-d9cb-469f-a165-70867728950e" {
		t.Fatalf("got %v %v", checked, err)
	}
	eleven := make([]string, 11)
	for index := range eleven {
		eleven[index] = ids[1]
	}
	if _, err := (AcknowledgementCheck{}).Checked(eleven); numberOf(err) != 6017 {
		t.Fatalf("got %v", err)
	}
	if _, err := (AcknowledgementCheck{}).Checked(nil); numberOf(err) != 6017 {
		t.Fatalf("got %v", err)
	}
	if _, err := (AcknowledgementCheck{}).Checked([]string{"x"}); numberOf(err) != 6018 {
		t.Fatalf("got %v", err)
	}
}

func TestDeviceKeysAreSafeFixedLengthHexInEightBuckets(t *testing.T) {
	key := DeviceKey{}.Of("b1")
	if !(DeviceKey{}).Valid(key) || len(key) != 25 || (DeviceKey{}).Valid("d"+strings.Repeat("g", 24)) {
		t.Fatalf("got %q", key)
	}
	buckets := map[string]bool{}
	for index := 0; index < 200; index++ {
		buckets[DeviceKey{}.Bucket(DeviceKey{}.Of(string(rune('a'+index%26))+strings.Repeat("q", index)))] = true
	}
	if len(buckets) != 8 {
		t.Fatalf("got %v", buckets)
	}
}

func TestSqlTextRefusesNamesThatAreNotSafe(t *testing.T) {
	if _, err := (SqlText{}).WaitingFor("slips; DROP", DeviceKey{}.Of("b1"), 0, 1); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if _, err := (SqlText{}).WaitingFor("slips", "d' OR 1=1", 0, 1); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	if _, err := (SqlText{}).Outbox("bodies", "ciphertext", 1); numberOf(err) != 6021 {
		t.Fatalf("got %v", err)
	}
	statement, err := SqlText{}.SlipByEnvelope("slips", "x' OR '1'='1")
	if err != nil || strings.Contains(statement.Sql, "OR") || statement.Parameters[0] != "x' OR '1'='1" {
		t.Fatalf("a value reached the SQL text: %+v", statement)
	}
}
