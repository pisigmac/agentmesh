package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

func TestDeriveAndRoundTripEncrypt(t *testing.T) {
	key, err := deriveKey("test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	s := &Store{gcm: gcm}
	ct, err := s.encrypt([]byte(`{"api_key":"sk-test"}`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := s.decrypt(ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"api_key":"sk-test"}` {
		t.Fatalf("got %s", plain)
	}
}

func TestHint(t *testing.T) {
	if hint("sk-abcdefghij") != "sk-a…ghij" {
		t.Fatalf("unexpected hint %q", hint("sk-abcdefghij"))
	}
}
