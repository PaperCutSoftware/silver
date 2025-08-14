package jsonsig

import (
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
