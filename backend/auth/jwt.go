/*
Copyright 2025 The Kube Bind Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	issuer     string
}

type Claims struct {
	Subject     string `json:"sub"`
	Issuer      string `json:"iss"`
	SessionID   string `json:"sid"`
	ClusterID   string `json:"cid"`
	RedirectURL string `json:"red,omitempty"`
	jwt.RegisteredClaims
}

func NewJWTService(issuer string) (*JWTService, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	return &JWTService{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		issuer:     issuer,
	}, nil
}

func (js *JWTService) GenerateToken(subject, oidcIssuer, sessionID, clusterID, redirectURL string, expiration time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		Subject:     subject,
		Issuer:      oidcIssuer,
		SessionID:   sessionID,
		ClusterID:   clusterID,
		RedirectURL: redirectURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    js.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiration)),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(js.privateKey)
}

func (js *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return js.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (js *JWTService) GetPublicKey() *rsa.PublicKey {
	return js.publicKey
}
