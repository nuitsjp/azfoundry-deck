package updates

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func testManifest() Manifest {
	return Manifest{
		AppID: "AzFoundryDeck", Version: "1.2.3", OS: "windows", Arch: "amd64",
		Filename: "azfoundrydeck-1.2.3-amd64-setup.exe", Size: 12345,
		SHA256: strings.Repeat("ab", 32), Notes: "Update details",
	}
}

func testSigningKeys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return public, private
}

func marshalEnvelope(t *testing.T, envelope Envelope) []byte {
	t.Helper()
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Sign arbitrary input without Sign's validation so Verify's external boundary
// is tested independently of the release command's checks.
func signedManifestInput(t *testing.T, manifest Manifest, key ed25519.PrivateKey) []byte {
	t.Helper()
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return marshalEnvelope(t, Envelope{
		Payload:   base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(key, payload)),
	})
}

func TestManifestSignatureAuthenticatesPayloadAndKey(t *testing.T) {
	public, private := testSigningKeys(t)
	want := testManifest()
	data, err := Sign(want, private)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Verify(data, public)
	if err != nil || got != want {
		t.Fatalf("Verify = %#v, %v; want %#v", got, err, want)
	}

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	tampered := envelope
	changed := want
	changed.Version = "9.9.9"
	payload, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	tampered.Payload = base64.StdEncoding.EncodeToString(payload)
	if _, err := Verify(marshalEnvelope(t, tampered), public); err == nil {
		t.Fatal("accepted a replaced payload with the original signature")
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil {
		t.Fatal(err)
	}
	signature[0] ^= 1
	tampered = envelope
	tampered.Signature = base64.StdEncoding.EncodeToString(signature)
	if _, err := Verify(marshalEnvelope(t, tampered), public); err == nil {
		t.Fatal("accepted a modified signature")
	}
	otherPublic, _ := testSigningKeys(t)
	if _, err := Verify(data, otherPublic); err == nil {
		t.Fatal("accepted a signature from a different key")
	}
}

func TestVerifyRejectsMalformedEnvelopeAndKey(t *testing.T) {
	public, private := testSigningKeys(t)
	data := signedManifestInput(t, testManifest(), private)
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		key  ed25519.PublicKey
	}{
		{"invalid JSON", []byte("{"), public},
		{"invalid payload encoding", marshalEnvelope(t, Envelope{Payload: "!", Signature: envelope.Signature}), public},
		{"invalid signature encoding", marshalEnvelope(t, Envelope{Payload: envelope.Payload, Signature: "!"}), public},
		{"short signature", marshalEnvelope(t, Envelope{Payload: envelope.Payload, Signature: base64.StdEncoding.EncodeToString([]byte{1})}), public},
		{"missing public key", data, nil},
		{"short public key", data, public[:len(public)-1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(tc.data, tc.key); err == nil {
				t.Fatal("accepted invalid update metadata")
			}
		})
	}
	for _, key := range []ed25519.PrivateKey{nil, private[:len(private)-1]} {
		if _, err := Sign(testManifest(), key); err == nil {
			t.Fatal("accepted an invalid signing key")
		}
	}
}

func TestVerifyEnforcesManifestSizeLimit(t *testing.T) {
	public, private := testSigningKeys(t)
	data := signedManifestInput(t, testManifest(), private)
	data = append(data, bytes.Repeat([]byte(" "), maxManifestSize-len(data))...)
	if _, err := Verify(data, public); err != nil {
		t.Fatalf("rejected metadata at the size limit: %v", err)
	}
	if _, err := Verify(append(data, ' '), public); err == nil {
		t.Fatal("accepted metadata above the size limit")
	}
}

func TestManifestRejectsUnsafeReleaseInputsEvenWhenSigned(t *testing.T) {
	public, private := testSigningKeys(t)
	for _, tc := range []struct {
		name   string
		change func(*Manifest)
	}{
		{"missing app ID", func(m *Manifest) { m.AppID = "" }},
		{"unsupported OS", func(m *Manifest) { m.OS = "linux" }},
		{"unsupported architecture", func(m *Manifest) { m.Arch = "386" }},
		{"missing filename", func(m *Manifest) { m.Filename = "" }},
		{"parent path", func(m *Manifest) { m.Filename = "../setup.exe" }},
		{"Windows parent path", func(m *Manifest) { m.Filename = `..\setup.exe` }},
		{"absolute path", func(m *Manifest) { m.Filename = `C:\setup.exe` }},
		{"alternate data stream", func(m *Manifest) { m.Filename = "setup.exe:payload.exe" }},
		{"NUL in filename", func(m *Manifest) { m.Filename = "setup\x00.exe" }},
		{"wrong extension", func(m *Manifest) { m.Filename = "setup.zip" }},
		{"filename above limit", func(m *Manifest) { m.Filename = strings.Repeat("a", 197) + ".exe" }},
		{"empty installer", func(m *Manifest) { m.Size = 0 }},
		{"negative installer size", func(m *Manifest) { m.Size = -1 }},
		{"installer above limit", func(m *Manifest) { m.Size = maxInstallerSize + 1 }},
		{"non-hex hash", func(m *Manifest) { m.SHA256 = strings.Repeat("g", 64) }},
		{"short hash", func(m *Manifest) { m.SHA256 = strings.Repeat("ab", 31) }},
		{"incomplete version", func(m *Manifest) { m.Version = "1.2" }},
		{"extra version part", func(m *Manifest) { m.Version = "1.2.3.4" }},
		{"leading zero in version", func(m *Manifest) { m.Version = "01.2.3" }},
		{"negative version part", func(m *Manifest) { m.Version = "1.-2.3" }},
		{"non-numeric version", func(m *Manifest) { m.Version = "v1.2.3" }},
		{"version above Windows limit", func(m *Manifest) { m.Version = "1.2.65536" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := testManifest()
			tc.change(&manifest)
			if _, err := Sign(manifest, private); err == nil {
				t.Fatal("release signing accepted invalid metadata")
			}
			if _, err := Verify(signedManifestInput(t, manifest, private), public); err == nil {
				t.Fatal("update verification trusted invalid metadata because it was signed")
			}
		})
	}
}

func TestManifestAcceptsSupportedBoundaryValues(t *testing.T) {
	public, private := testSigningKeys(t)
	for _, tc := range []struct {
		name     string
		arch     string
		version  string
		size     int64
		filename string
	}{
		{"minimum", "amd64", "0.0.0", 1, "a.exe"},
		{"maximum", "arm64", "65535.65535.65535", maxInstallerSize, strings.Repeat("a", 196) + ".exe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := testManifest()
			manifest.Arch, manifest.Version, manifest.Size, manifest.Filename = tc.arch, tc.version, tc.size, tc.filename
			data, err := Sign(manifest, private)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := Verify(data, public); err != nil || got != manifest {
				t.Fatalf("Verify = %#v, %v; want %#v", got, err, manifest)
			}
		})
	}
}

func TestCompareVersionUsesNumericReleaseOrdering(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.2", "1.2.3", -1},
		{"1.10.0", "1.9.999", 1},
		{"1.9.999", "1.10.0", -1},
		{"2.0.0", "1.65535.65535", 1},
		{"0.65535.65535", "1.0.0", -1},
		{"0.0.0", "0.0.0", 0},
		{"65535.65535.65535", "65535.65535.65535", 0},
	} {
		t.Run(tc.left+" vs "+tc.right, func(t *testing.T) {
			if got, err := CompareVersion(tc.left, tc.right); err != nil || got != tc.want {
				t.Fatalf("CompareVersion = %d, %v; want %d", got, err, tc.want)
			}
		})
	}
	for _, invalid := range []string{"", "1.2", "1.2.3.4", "1..3", "01.2.3", "1.-2.3", "1.2.x", "1.2.65536"} {
		t.Run("invalid "+invalid, func(t *testing.T) {
			if _, err := CompareVersion(invalid, "1.2.3"); err == nil {
				t.Fatal("accepted an invalid new version")
			}
			if _, err := CompareVersion("1.2.3", invalid); err == nil {
				t.Fatal("accepted an invalid installed version")
			}
		})
	}
}
