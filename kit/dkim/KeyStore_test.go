//go:debug rsa1024min=0

package dkim

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testPem(t *testing.T, blockType string, der []byte) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
}

func testPkcs8(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return testPem(t, "PRIVATE KEY", der)
}

func testWriteSecret(t *testing.T, root string, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name)+".pem")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestKeyStoreReadsEveryPemFormatAndSigns(t *testing.T) {
	cases := map[string][]byte{
		"pkcs1 rsa":     testPem(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(testRSA(t))),
		"pkcs8 rsa":     testPkcs8(t, testRSA(t)),
		"pkcs8 ed25519": testPkcs8(t, testEd25519(t)),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			testWriteSecret(t, root, "dkim/example.com/s1", data)
			key, err := KeyStore{Source: FolderSecretSource{Root: root}}.Key("Example.com", "S1")
			if err != nil {
				t.Fatal(err)
			}
			if key.Domain != "Example.com" || key.Selector != "S1" {
				t.Errorf("key names = %q %q", key.Domain, key.Selector)
			}
			record, err := DnsRecord{}.Text(key)
			if err != nil {
				t.Fatal(err)
			}
			public, err := testPublicFromRecord(record)
			if err != nil {
				t.Fatal(err)
			}
			signed, err := Signer{}.Sign(SignRequest{Message: testMessage("From: a@example.com\r\n", "x\r\n"), Key: key, Time: time.Unix(1, 0)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := testVerify(signed, public); err != nil {
				t.Fatalf("verify with the DNS record key: %v", err)
			}
		})
	}
}

func TestKeyStoreSelectorsCoexist(t *testing.T) {
	root := t.TempDir()
	testWriteSecret(t, root, "dkim/example.com/old", testPkcs8(t, testRSA(t)))
	testWriteSecret(t, root, "dkim/example.com/new", testPkcs8(t, testEd25519(t)))
	store := KeyStore{Source: FolderSecretSource{Root: root}}
	old, err := store.Key("example.com", "old")
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := store.Key("example.com", "new")
	if err != nil {
		t.Fatal(err)
	}
	if old.Signer.Public().(*rsa.PublicKey) == nil || len(fresh.Signer.Public().(ed25519.PublicKey)) != ed25519.PublicKeySize {
		t.Error("wrong key types")
	}
}

type testSource struct {
	data []byte
	err  error
}

func (source testSource) Secret(string) ([]byte, error) { return source.data, source.err }

func TestKeyStoreErrorsNeverContainKeyMaterial(t *testing.T) {
	small, err := rsa.GenerateKey(rand.Reader, 512)
	if err != nil {
		t.Fatal(err)
	}
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	x25519, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	valid := testPkcs8(t, testRSA(t))
	corrupt := testPem(t, "PRIVATE KEY", []byte("this is not der but it is secret-ish"))
	cases := map[string]struct {
		data []byte
		want any
	}{
		"small rsa":     {testPem(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(small)), &UnsupportedKey{}},
		"ecdsa":         {testPkcs8(t, ecdsaKey), &UnsupportedKey{}},
		"x25519":        {testPkcs8(t, x25519), &UnsupportedKey{}},
		"not pem":       {[]byte("raw secret bytes"), &UnsupportedKey{}},
		"wrong type":    {testPem(t, "CERTIFICATE", []byte("abcdefghijklmnopqrstuvwxyz")), &UnsupportedKey{}},
		"corrupt pkcs8": {corrupt, &UnsupportedKey{}},
		"corrupt pkcs1": {testPem(t, "RSA PRIVATE KEY", valid[40:200]), &UnsupportedKey{}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := KeyStore{Source: testSource{data: test.data}}.Key("example.com", "s1")
			if !errors.As(err, test.want) {
				t.Fatalf("err = %T %v", err, err)
			}
			if name == "small rsa" && !strings.Contains(err.Error(), "512 bits") {
				t.Errorf("small key not refused by the size check: %v", err)
			}
			block, _ := pem.Decode(test.data)
			text := err.Error()
			if block != nil && strings.Contains(text, base64.StdEncoding.EncodeToString(block.Bytes)[:16]) {
				t.Error("error contains key bytes")
			}
			if strings.Contains(text, "secret-ish") || strings.Contains(text, "raw secret") {
				t.Error("error contains secret text")
			}
		})
	}
}

func TestKeyStoreMissingAndFailingSources(t *testing.T) {
	_, err := KeyStore{Source: FolderSecretSource{Root: t.TempDir()}}.Key("example.com", "s9")
	var missing MissingKey
	if !errors.As(err, &missing) || missing.Domain != "example.com" || missing.Selector != "s9" {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(missing.Error(), "s9") {
		t.Error(missing.Error())
	}
	failure := errors.New("store offline")
	_, err = KeyStore{Source: testSource{err: failure}}.Key("example.com", "s1")
	if !errors.Is(err, failure) {
		t.Fatalf("err = %v", err)
	}
	_, err = KeyStore{Source: testSource{err: failure}}.Key("../etc", "s1")
	var invalid InvalidKeyName
	if !errors.As(err, &invalid) || !strings.Contains(invalid.Error(), "../etc") {
		t.Fatalf("err = %v", err)
	}
}

func TestFolderSecretSourceRefusesUnsafeNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", "../x", "dkim/../../x", "/etc/passwd", "dkim//x", "dkim/Example.com/s1", "dkim/a b/s1", "dkim/.hidden/s1", "dkim/a/", `dkim\a\b`, "dkim/a..b/s1"} {
		t.Run(name, func(t *testing.T) {
			_, err := FolderSecretSource{Root: root}.Secret(name)
			var unsafe UnsafeSecretName
			if !errors.As(err, &unsafe) || unsafe.Name != name {
				t.Fatalf("err = %v", err)
			}
			if !strings.Contains(unsafe.Error(), "not safe") {
				t.Error(unsafe.Error())
			}
		})
	}
}

func TestFolderSecretSourceReads(t *testing.T) {
	root := t.TempDir()
	testWriteSecret(t, root, "dkim/example.com/s1", []byte("pem"))
	data, err := FolderSecretSource{Root: root}.Secret("dkim/example.com/s1")
	if err != nil || string(data) != "pem" {
		t.Fatalf("data = %q err = %v", data, err)
	}
	_, err = FolderSecretSource{Root: root}.Secret("dkim/example.com/s2")
	var missing MissingSecret
	if !errors.As(err, &missing) || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(missing.Error(), "s2") {
		t.Fatalf("err = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "dkim", "example.com", "folder.pem"), 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = FolderSecretSource{Root: root}.Secret("dkim/example.com/folder")
	if err == nil || errors.As(err, &missing) {
		t.Fatalf("err = %v, want a read error", err)
	}
}

func TestEnvironmentSecretSource(t *testing.T) {
	source := EnvironmentSecretSource{Prefix: "MESAIL_SECRET_"}
	if got := source.VariableName("dkim/example.com/s1"); got != "MESAIL_SECRET_DKIM_EXAMPLE_COM_S1" {
		t.Fatalf("VariableName = %q", got)
	}
	t.Setenv("MESAIL_SECRET_DKIM_EXAMPLE_COM_S1", string(testPkcs8(t, testEd25519(t))))
	key, err := KeyStore{Source: source}.Key("example.com", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := key.Signer.Public().(ed25519.PublicKey); !ok {
		t.Error("not ed25519")
	}
	_, err = KeyStore{Source: source}.Key("example.com", "s2")
	if !errors.As(err, &MissingKey{}) {
		t.Fatalf("err = %v", err)
	}
}

func TestDnsRecordText(t *testing.T) {
	rsaRecord, err := DnsRecord{}.Text(testKeys(t)["rsa"])
	if err != nil || !strings.HasPrefix(rsaRecord, "v=DKIM1; k=rsa; p=MII") {
		t.Fatalf("record = %q err = %v", rsaRecord, err)
	}
	edKey := testEd25519(t)
	edRecord, err := DnsRecord{}.Text(Key{Domain: "example.com", Selector: "e", Signer: edKey})
	want := "v=DKIM1; k=ed25519; p=" + base64.StdEncoding.EncodeToString(edKey.Public().(ed25519.PublicKey))
	if err != nil || edRecord != want {
		t.Fatalf("record = %q err = %v", edRecord, err)
	}
	if _, err := (DnsRecord{}).Text(Key{}); !errors.As(err, &UnsupportedKey{}) {
		t.Fatalf("err = %v", err)
	}
}

func TestErrorTexts(t *testing.T) {
	errorsToCheck := []error{
		DomainMismatch{From: "a@b.test", Key: "example.com"},
		UnsupportedKey{Reason: "r"},
		MalformedMessage{Reason: "m"},
	}
	for _, err := range errorsToCheck {
		if !strings.HasPrefix(err.Error(), "dkim: ") {
			t.Errorf("%q", err.Error())
		}
	}
}
