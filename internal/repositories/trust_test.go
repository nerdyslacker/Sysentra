package repositories

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
)

func TestParseKeyIdentityReturnsSignerAndFingerprint(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	publicKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded})
	plist := `<?xml version="1.0"?><plist><dict><key>public-key</key><data>` + base64.StdEncoding.EncodeToString(publicKey) + `</data><key>public-key-size</key><integer>1024</integer><key>signature-by</key><string>Example signer</string><key>signature-type</key><string>rsa</string></dict></plist>`
	identity, err := parseKeyIdentity([]byte(plist))
	if err != nil || identity.Signer != "Example signer" || identity.Type != "rsa" || identity.Bits != "1024" || strings.Count(identity.Fingerprint, ":") != 31 {
		t.Fatalf("identity=%#v err=%v", identity, err)
	}
}
