// Package jsonsig provides a simple way to sign and verify JSON payloads using
// Ed25519 signatures. The signature is attached to the JSON payload (most be
// a JSON object) in a "signature" field.
package jsonsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/go-jose/go-jose/v4"
	"github.com/gowebpki/jcs"
)

// GenerateKeys creates a new Ed25519 key pair.
// It returns the public key, private key (both base64 encoded), and an error if one occurred.
func GenerateKeys() (string, string, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}

	privateKeyB64 := base64.StdEncoding.EncodeToString(privateKey)
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)

	return publicKeyB64, privateKeyB64, nil
}

// Sign takes a JSON payload and a base64 encoded private key, and returns the
// signed JSON payload. The signature is added to the JSON payload in a "signature" field.
func Sign(payload []byte, privateKeyB64 string) ([]byte, error) {
	// Validate we've got a valid JSON object.
	var m map[string]interface{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, fmt.Errorf("payload must be a JSON object (e.g {...}): %w", err)
	}

	// Check if the payload does NOT have a signature.
	if _, ok := m["signature"]; ok {
		return nil, fmt.Errorf("payload already contains a 'signature' field; maybe it is already signed")
	}

	// Canonicalize the payload that we will sign.
	canonicalPayload, err := jcs.Transform(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize payload: %w", err)
	}

	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, err
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.EdDSA, Key: ed25519.PrivateKey(privateKey)}, (&jose.SignerOptions{}).WithHeader("kid", "silver-service-key-v1"))
	if err != nil {
		return nil, err
	}

	jws, err := signer.Sign(canonicalPayload)
	if err != nil {
		return nil, err
	}

	// Get the detached compact serialization, which is the signature.
	compact, err := jws.DetachedCompactSerialize()
	if err != nil {
		return nil, err
	}

	// Now, we add the signature to the map.
	m["signature"] = compact

	return json.MarshalIndent(m, "", "    ")
}

// Verify takes a signed JSON payload and a base64 encoded public key, and returns
// true if the signature is valid.
func Verify(signedPayload []byte, publicKeyB64 string) (bool, error) {
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return false, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(signedPayload, &m); err != nil {
		return false, fmt.Errorf("payload must be a JSON object: %w", err)
	}

	compact, ok := m["signature"].(string)
	if !ok {
		return false, fmt.Errorf("invalid signature format: 'signature' field missing or not a string")
	}

	delete(m, "signature")

	// To verify, we need to canonicalize the payload as it was before signing.
	// This means marshaling the map (without signature) back to JSON, and then
	// canonicalizing that.
	unsignedPayload, err := json.Marshal(m)
	if err != nil {
		return false, fmt.Errorf("failed to marshal unsigned payload: %w", err)
	}

	canonicalPayload, err := jcs.Transform(unsignedPayload)
	if err != nil {
		return false, fmt.Errorf("failed to canonicalize payload for verification: %w", err)
	}

	object, err := jose.ParseDetached(compact, canonicalPayload, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil {
		return false, fmt.Errorf("failed to parse detached signature: %w", err)
	}

	_, err = object.Verify(ed25519.PublicKey(publicKey))
	if err != nil {
		return false, fmt.Errorf("signature verification failed: %w", err)
	}

	return true, nil
}
