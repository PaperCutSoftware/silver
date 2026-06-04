// SILVER - Service Wrapper
// Auto Updater
//
// Copyright (c) 2014-2026 PaperCut Software http://www.papercut.com/
// Use of this source code is governed by an MIT or GPL Version 2 license.
// See the project's LICENSE file for more information.
//

package jsonsig

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	// Generate a new key pair for testing.
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Create a sample JSON payload.
	payload := []byte(`{"foo": "bar"}`)

	// Sign the payload.
	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Verify the signature.
	valid, err := Verify(signedPayload, publicKey)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}

	if !valid {
		t.Error("Signature should be valid, but it was not.")
	}
}

func TestVerificationFailureWithDifferentPublicKey(t *testing.T) {
	// Generate two different key pairs.
	_, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	publicKey2, _, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Create a sample JSON payload.
	payload := []byte(`{"foo": "bar"}`)

	// Sign the payload with the first private key.
	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Try to verify the signature with the second public key.
	valid, err := Verify(signedPayload, publicKey2)
	if err == nil {
		t.Error("Verification should have failed, but it did not.")
	}

	if valid {
		t.Error("Signature should be invalid, but it was considered valid.")
	}
}

func TestVerificationFailureWithTamperedPayload(t *testing.T) {
	// Generate a new key pair for testing.
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Create a sample JSON payload.
	payload := []byte(`{"foo": "bar"}`)

	// Sign the payload.
	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Tamper with the signed payload.
	signedPayload[0] = 'A'

	// Try to verify the signature of the tampered payload.
	valid, err := Verify(signedPayload, publicKey)
	if err == nil {
		t.Error("Verification should have failed, but it did not.")
	}

	if valid {
		t.Error("Signature should be invalid, but it was considered valid.")
	}
}

func TestSignFailureWithExistingSignature(t *testing.T) {
	// Generate a new key pair for testing.
	_, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Create a sample JSON payload that already has a signature field.
	payload := []byte(`{"foo": "bar", "signature": "dummy"}`)

	// Try to sign the payload.
	_, err = Sign(payload, privateKey)
	if err == nil {
		t.Error("Signing should have failed because a signature field already exists, but it did not.")
	}
}

func TestLargeIntegerIncompatibility(t *testing.T) {
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	// Large integer payload, the value must be larger than 52 bits integer.
	payload := []byte(`{"value": 12345678901234567890}`)

	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	if !strings.Contains(string(signedPayload), "12345678901234567890") {
		t.Errorf("Precision lost: expected signed payload to contain 12345678901234567890, but got:\n%s", string(signedPayload))
	}

	valid, err := Verify(signedPayload, publicKey)
	if err != nil {
		t.Fatalf("Failed to verify: %v", err)
	}
	if !valid {
		t.Error("Verification failed for large integer payload")
	}
}

func TestDuplicateKeyBypass(t *testing.T) {
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}

	payload := []byte(`{"foo": "bar"}`)

	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Now we tamper with the signed payload by inserting a duplicate "foo" key with a different value
	// at the beginning of the JSON object.
	// Original is something like:
	// {
	//     "foo": "bar",
	//     "signature": "..."
	// }
	// We make it:
	// {
	//     "foo": "malicious",
	//     "foo": "bar",
	//     "signature": "..."
	// }
	// This is a malicious payload that should fail verification, or if verified, the signature should protect the whole payload bytes.
	// But let's see if Verify returns true.
	tamperedPayload := []byte(`{
"foo": "malicious",
"foo": "bar",
"signature": "`)

	// Extract signature from signedPayload
	var m map[string]any
	if err := json.Unmarshal(signedPayload, &m); err != nil {
		t.Fatalf("Failed to parse signedPayload: %v", err)
	}
	sig, _ := m["signature"].(string)

	tamperedPayload = append(tamperedPayload, []byte(sig)...)
	tamperedPayload = append(tamperedPayload, []byte(`"}`)...)

	valid, err := Verify(tamperedPayload, publicKey)
	if err == nil && valid {
		t.Error("VULNERABILITY: Verify returned true for a tampered payload with duplicate keys!")
	}
}

func TestVerifyErrorWithInvalidLengths(t *testing.T) {
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	payload := []byte(`{"foo": "bar"}`)
	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	t.Run("invalid short public key", func(t *testing.T) {
		shortPublicKey := base64.StdEncoding.EncodeToString([]byte("too_short"))
		_, err = Verify(signedPayload, shortPublicKey)
		if err == nil {
			t.Error("Expected error for short public key, got nil")
		}
	})

	// Invalid short signature in the payload (e.g., "dG9vX3Nob3J0" = "too_short")
	t.Run("invalid short signature", func(t *testing.T) {
		tamperedPayload := []byte(`{"foo": "bar", "signature": "dG9vX3Nob3J0"}`)
		_, err = Verify(tamperedPayload, publicKey)
		if err == nil {
			t.Error("Expected error for short signature, got nil")
		}
	})
}

func TestBase64PaddingMalleability(t *testing.T) {
	publicKey, privateKey, err := GenerateKeys()
	if err != nil {
		t.Fatalf("Failed to generate keys: %v", err)
	}
	payload := []byte(`{"foo": "bar"}`)
	signedPayload, err := Sign(payload, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	// Parse the signature from signedPayload
	var m map[string]any
	if err := json.Unmarshal(signedPayload, &m); err != nil {
		t.Fatalf("Failed to parse signedPayload: %v", err)
	}
	sig, _ := m["signature"].(string)

	// Since a 64-byte signature encoded in standard base64 has 88 characters ending with "==",
	// the character at index 85 (immediately preceding "==") has 4 unused bits.
	if len(sig) < 88 || !strings.HasSuffix(sig, "==") {
		t.Fatalf("Expected signature to be at least 88 chars and end with ==, got %s (len %d)", sig, len(sig))
	}

	lastCharIdx := len(sig) - 3 // character right before "=="
	origChar := sig[lastCharIdx]

	// Find another character in the same 16-character group
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	origIdx := strings.IndexByte(chars, origChar)
	if origIdx == -1 {
		t.Fatalf("Character %q not found in base64 alphabet", origChar)
	}

	// Change the character to a different one in the same 16-character boundary.
	// For example, flip the least significant bit of origIdx.
	newIdx := origIdx ^ 1
	newChar := chars[newIdx]

	// Create malleable signature
	sigRunes := []rune(sig)
	sigRunes[lastCharIdx] = rune(newChar)
	malleableSig := string(sigRunes)

	// Build the tampered payload with the malleable signature
	m["signature"] = malleableSig
	tamperedPayload, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Failed to marshal tampered payload: %v", err)
	}

	// Try to verify
	valid, err := Verify(tamperedPayload, publicKey)
	if err == nil && valid {
		t.Error("VULNERABILITY: Verify returned true for malleable base64 signature!")
	}
}
