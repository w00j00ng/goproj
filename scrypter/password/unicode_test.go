package password_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/w00j00ng/goproj/scrypter/password"
)

func TestUnicodeCompatibility(t *testing.T) {
	data, err := os.ReadFile("testdata/unicode.json")
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Fixtures []struct {
			Name       string
			Plain      string
			Normalized string
			Hash       string
			Supported  bool
		}
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Fixtures) == 0 {
		t.Fatal("no Unicode fixtures")
	}
	for _, tt := range document.Fixtures {
		t.Run(tt.Name, func(t *testing.T) {
			match, err := password.Verify(tt.Hash, tt.Plain)
			if !tt.Supported {
				if match || err == nil || !strings.Contains(err.Error(), "unsupported password normalization") {
					t.Fatalf("Verify()=(%v,%v); want explicit normalization error", match, err)
				}
				hash, err := password.Hash(tt.Plain)
				if hash != "" || err == nil || !strings.Contains(err.Error(), "unsupported password normalization") {
					t.Fatalf("Hash()=(%q,%v); want explicit normalization error", hash, err)
				}
				return
			}
			if !match || err != nil {
				t.Fatalf("Node fixture Verify()=(%v,%v)", match, err)
			}
			hash, err := password.Hash(tt.Plain)
			if err != nil {
				t.Fatal(err)
			}
			match, err = password.Verify(hash, tt.Normalized)
			if !match || err != nil {
				t.Fatalf("normalized password Verify()=(%v,%v)", match, err)
			}
			if strings.Contains(tt.Plain, "\u034f") {
				match, err = password.Verify(tt.Hash, strings.ReplaceAll(tt.Plain, "\u034f", ""))
				if match {
					t.Fatal("original U+034F was discarded")
				}
			}
		})
	}
}

func TestInvalidUTF8Password(t *testing.T) {
	for _, plain := range []string{"\xff", "a\xc0\xaf", "test\xe2\x82"} {
		t.Run(plain, func(t *testing.T) {
			hash, err := password.Hash(plain)
			if hash != "" || err == nil || !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("Hash()=(%q,%v); want UTF-8 error", hash, err)
			}
			match, err := password.Verify(exampleHash, plain)
			if match || err == nil || !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("Verify()=(%v,%v); want UTF-8 error", match, err)
			}
		})
	}
}
