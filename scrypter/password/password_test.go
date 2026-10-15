package password_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/scrypter/password"
)

// Fixtures were computed independently with Node.js crypto.scryptSync using
// Better Auth's default parameters and the salt string as UTF-8 text.
const exampleHash = "00112233445566778899aabbccddeeff:69a78a4e2f2c0b88e195ee2b95216789cd67200921e02827f6038ecf545b9e9b1e1a9d6d6fa8c28f22432cbfdec55fbfab5e8289e0529924060c6a8e504e32e7"
const normalizedHash = "00112233445566778899aabbccddeeff:fed54c3cb84e24242dd241817bdfdbdcd0b9d668828f0d9f112bc7973526c9b3f4706d481c3067ed0d1cb23ea7a7eeb7c80315f67af42bbe3b5d6cfb295314ee"
const unicodeHash = "00112233445566778899aabbccddeeff:0b9a60406df7ee65e6eba05dfdd3ef650a9576a2fca7ae942670f27f6fa12326b1f7544136ee9e43e0b2a9b5e496d03139bc946153872dbad2a0aa8c2f601a87"
const emptyHash = "00112233445566778899aabbccddeeff:497fd99302c442f6627fee539466649b1d40a6c71680cb681c10927ac69fddbfdfaab89fc61bc62faaa91b5099235a7e64362d00e1134a19ce7d03507e9178fa"
const longHash = "00112233445566778899aabbccddeeff:f582e6ad5f21512248633955ac4e52a586caea6019a98d08f44577a674fb460ec98cecb7922cb47d165f5a9f63082a4e4651cd64b3f3db30d663eea8c88c7e37"

func TestVerify(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		plain   string
		match   bool
		wantErr bool
	}{
		{"Better Auth fixture", exampleHash, "example-password", true, false},
		{"different password", exampleHash, "wrong-password", false, false},
		{"preserve whitespace", exampleHash, " example-password ", false, false},
		{"NFKC fullwidth", normalizedHash, "Ｐａｓｓｗｏｒｄ１２３！", true, false},
		{"NFKC decomposed", unicodeHash, "cafe\u0301", true, false},
		{"empty password", emptyHash, "", true, false},
		{"long password", longHash, strings.Repeat("a", 100), true, false},
		{"changed salt", "ff" + exampleHash[2:], "example-password", false, false},
		{"salt text case matters", strings.ToUpper(exampleHash[:32]) + exampleHash[32:], "example-password", false, false},
		{"changed key", exampleHash[:len(exampleHash)-1] + "0", "example-password", false, false},
		{"empty hash", "", "example-password", false, true},
		{"missing separator", strings.ReplaceAll(exampleHash, ":", ""), "example-password", false, true},
		{"extra field", exampleHash + ":extra", "example-password", false, true},
		{"empty salt", exampleHash[32:], "example-password", false, true},
		{"short salt", exampleHash[2:], "example-password", false, true},
		{"invalid salt hex", "zz" + exampleHash[2:], "example-password", false, true},
		{"short key", exampleHash[:len(exampleHash)-2], "example-password", false, true},
		{"invalid key hex", exampleHash[:len(exampleHash)-2] + "zz", "example-password", false, true},
		{"unsupported format", "$scrypt$ln=17,r=8,p=1$salt$key", "example-password", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := password.Verify(tt.hash, tt.plain)
			if match != tt.match || (err != nil) != tt.wantErr {
				t.Fatalf("Verify()=(%v, %v); want match=%v, error=%v", match, err, tt.match, tt.wantErr)
			}
		})
	}
}

func TestHash(t *testing.T) {
	for _, plain := range []string{"example-password", " password ", "Ｐａｓｓｗｏｒｄ１２３！", "", strings.Repeat("a", 100)} {
		t.Run(plain, func(t *testing.T) {
			first, err := password.Hash(plain)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(first, ":")
			if len(parts) != 2 || len(parts[0]) != 32 || len(parts[1]) != 128 {
				t.Fatalf("unexpected hash format: %q", first)
			}
			for _, part := range parts {
				if _, err := hex.DecodeString(part); err != nil {
					t.Fatalf("hash field is not hex: %v", err)
				}
			}
			match, err := password.Verify(first, plain)
			if err != nil || !match {
				t.Fatalf("generated hash did not verify: match=%v, error=%v", match, err)
			}
			second, err := password.Hash(plain)
			if err != nil {
				t.Fatal(err)
			}
			if first[:32] == second[:32] {
				t.Fatal("repeated hashing reused the salt")
			}
		})
	}
}
