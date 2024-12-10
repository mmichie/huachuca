package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
)

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid     string   `json:"kid"`      // Key ID
	Kty     string   `json:"kty"`      // Key type (RSA)
	Alg     string   `json:"alg"`      // Algorithm (RS256)
	Use     string   `json:"use"`      // Use (sig - signature)
	N       string   `json:"n"`        // Modulus
	E       string   `json:"e"`        // Exponent
	X5c     []string `json:"x5c"`      // X.509 certificate chain
	X5t     string   `json:"x5t"`      // X.509 certificate SHA-1 thumbprint
	X5tS256 string   `json:"x5t#S256"` // X.509 certificate SHA-256 thumbprint
}

// Convert RSA public key to JWK format
func rsaPublicKeyToJWK(publicKey *rsa.PublicKey, kid string) (*JWK, error) {
	// Convert modulus to base64URL
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())

	// Convert exponent to base64URL
	eBytes := big.NewInt(int64(publicKey.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	// Create X.509 public key
	publicKeyDer, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}

	// Base64 encode the DER directly for x5c
	x5c := []string{base64.StdEncoding.EncodeToString(publicKeyDer)}

	return &JWK{
		Kid: kid,
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   n,
		E:   e,
		X5c: x5c,
	}, nil
}

// Add JWKSHandler to Server struct
func (s *Server) handleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get public key from token manager
	publicKey := s.tokenManager.GetPublicKey()

	// Convert to JWK
	jwk, err := rsaPublicKeyToJWK(publicKey, "default-key")
	if err != nil {
		s.logger.Error("failed to convert public key to JWK", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create JWKS
	jwks := JWKS{
		Keys: []JWK{*jwk},
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	// Write response
	if err := json.NewEncoder(w).Encode(jwks); err != nil {
		s.logger.Error("failed to encode JWKS response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
