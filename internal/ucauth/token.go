package ucauth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"strings"
	"time"
)

type InfOneConfig struct {
	Issuer        string
	ClientID      string
	PublicKeyPath string
}

type InfOneVerifier struct {
	issuer   string
	clientID string
	key      verificationKey
	now      func() time.Time
}

func NewInfOneVerifier(cfg InfOneConfig) (*InfOneVerifier, error) {
	path := strings.TrimSpace(cfg.PublicKeyPath)
	if path == "" {
		return nil, errors.New("authn infone public key path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(data)
	if block == nil || block.Type != "PUBLIC KEY" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("authn infone public key must contain one PEM block")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("authn infone public key must use PKIX format")
	}
	key, err := newVerificationKey(publicKey)
	if err != nil {
		return nil, err
	}
	return &InfOneVerifier{
		issuer:   strings.TrimSpace(cfg.Issuer),
		clientID: cfg.ClientID,
		key:      key,
		now:      time.Now,
	}, nil
}

func newVerificationKey(publicKey any) (verificationKey, error) {
	switch key := publicKey.(type) {
	case *rsa.PublicKey:
		if key.N == nil || key.N.Sign() <= 0 || key.E <= 1 {
			return verificationKey{}, ErrInvalidToken
		}
		return verificationKey{alg: "RS256", rsa: key}, nil
	case *ecdsa.PublicKey:
		if key.Curve != elliptic.P256() {
			return verificationKey{}, errors.New("authn infone ECDSA public key must use P-256")
		}
		return verificationKey{alg: "ES256", ecdsa: key}, nil
	default:
		return verificationKey{}, errors.New("authn infone public key must be RSA or P-256 ECDSA")
	}
}

func (v *InfOneVerifier) VerifyToken(token string) (Identity, error) {
	claims, err := v.verifiedClaims(token)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Subject: claims.Subject, Name: firstNonEmpty(claims.PreferredUsername, claims.Username, claims.Name), Email: claims.Email}, nil
}
func (v *InfOneVerifier) verifiedClaims(token string) (jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, ErrInvalidToken
	}

	headerBytes, err := decodeJWTPart(parts[0])
	if err != nil {
		return jwtClaims{}, ErrInvalidToken
	}
	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil || !v.key.verify(header.Alg, parts[0]+"."+parts[1], parts[2]) {
		return jwtClaims{}, ErrInvalidToken
	}

	payloadBytes, err := decodeJWTPart(parts[1])
	if err != nil {
		return jwtClaims{}, ErrInvalidToken
	}
	var claims jwtClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil || v.validateClaims(claims) != nil {
		return jwtClaims{}, ErrInvalidToken
	}

	return claims, nil
}

func (v *InfOneVerifier) validateClaims(claims jwtClaims) error {
	if v.issuer != "" && claims.Issuer != v.issuer {
		return ErrInvalidToken
	}
	if claims.Subject == "" || claims.ClientID != v.clientID || claims.TokenType != "access" {
		return ErrInvalidToken
	}
	now := v.now().Unix()
	if claims.ExpiresAt == 0 || now >= claims.ExpiresAt {
		return ErrInvalidToken
	}
	if claims.NotBefore != 0 && now < claims.NotBefore {
		return ErrInvalidToken
	}
	return nil
}

type verificationKey struct {
	alg   string
	rsa   *rsa.PublicKey
	ecdsa *ecdsa.PublicKey
}

func (k verificationKey) verify(alg, signed, signature string) bool {
	if alg != k.alg {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	digest := sha256.Sum256([]byte(signed))
	switch alg {
	case "RS256":
		return k.rsa != nil && rsa.VerifyPKCS1v15(k.rsa, crypto.SHA256, digest[:], sig) == nil
	case "ES256":
		if k.ecdsa == nil || len(sig) != 64 {
			return false
		}
		return ecdsa.Verify(k.ecdsa, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]))
	default:
		return false
	}
}

func decodeJWTPart(part string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(part)
}

type jwtHeader struct {
	Alg string `json:"alg"`
}

type jwtClaims struct {
	ClientID          string `json:"client_id"`
	TokenType         string `json:"type"`
	Issuer            string `json:"iss"`
	Subject           string `json:"sub"`
	ExpiresAt         int64  `json:"exp"`
	NotBefore         int64  `json:"nbf,omitempty"`
	Name              string `json:"name,omitempty"`
	Username          string `json:"username,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
	Email             string `json:"email,omitempty"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var ErrInvalidToken = errors.New("invalid UC access token")

type Identity struct {
	Subject string `json:"sub"`
	Name    string `json:"display_name"`
	Email   string `json:"email"`
}
