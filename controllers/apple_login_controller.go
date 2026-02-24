package controllers

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AppleLogin(c *gin.Context) {
	var input struct {
		IdentityToken string `json:"identityToken"`
	}

	if err := c.ShouldBindJSON(&input); err != nil || input.IdentityToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identityToken required"})
		return
	}

	keys, err := fetchApplePublicKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch Apple keys"})
		return
	}

	token, err := jwt.Parse(input.IdentityToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}

		kid, ok := t.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid")
		}

		key := keys[kid]
		if key == nil {
			return nil, errors.New("invalid kid")
		}

		return key, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Apple token"})
		return
	}

	claims := token.Claims.(jwt.MapClaims)

	email, _ := claims["email"].(string)
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email not found in token"})
		return
	}

	appleUserID, ok := claims["sub"].(string)
	if !ok || appleUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing Apple user ID"})
		return
	}

	// Check if user exists
	var user models.User
	result := database.DB.Where("email = ?", email).First(&user)
	if result.Error != nil {
		// User does not exist, create one
		user = models.User{
			Name:     "", // optional: set to "" or "Apple User"
			Email:    email,
			Password: "", // optional: leave blank or store "apple"
		}
		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
	}

	// Generate JWT
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not configured"})
		return
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
	})

	tokenString, err := jwtToken.SignedString([]byte(secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
	})
}

func fetchApplePublicKeys() (map[string]*rsa.PublicKey, error) {
	resp, err := http.Get("https://appleid.apple.com/auth/keys")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey)

	for _, k := range body.Keys {
		nBytes, _ := base64.RawURLEncoding.DecodeString(k.N)
		eBytes, _ := base64.RawURLEncoding.DecodeString(k.E)

		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		keys[k.Kid] = &rsa.PublicKey{
			N: n,
			E: e,
		}
	}

	return keys, nil
}
