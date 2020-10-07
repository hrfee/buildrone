package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func hash(password string) (hash string, err error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	hash = string(bytes)
	return
}

func checkHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (app *appContext) buildAuth() gin.HandlerFunc {
	return app.validateBuildToken
}

func (app *appContext) webAuth() gin.HandlerFunc {
	return app.validateWebToken
}

// newBuildToken returns a web token as well as a refresh token, which can be used to obtain new tokens.
func newBuildToken(namespace, name, secret string) (string, string, error) {
	var token, refresh string
	claims := jwt.MapClaims{
		"valid":     true,
		"namespace": namespace,
		"repo":      name,
		"exp":       strconv.FormatInt(time.Now().Add(time.Minute*20).Unix(), 10),
		"type":      "bearer",
	}
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tk.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	claims["exp"] = strconv.FormatInt(time.Now().Add(time.Hour*time.Duration(24*TOKEN_PERIOD)).Unix(), 10)
	claims["type"] = "refresh"
	tk = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refresh, err = tk.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	return token, refresh, nil
}

func newWebTokens() (string, string, error) {
	var token, refresh string
	claims := jwt.MapClaims{
		"valid": true,
		"exp":   strconv.FormatInt(time.Now().Add(time.Minute*20).Unix(), 10),
		"type":  "bearer",
	}

	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tk.SignedString([]byte(os.Getenv("BUILDRONE_WEBSECRET")))
	if err != nil {
		return "", "", err
	}
	claims["exp"] = strconv.FormatInt(time.Now().Add(time.Hour*24).Unix(), 10)
	claims["type"] = "refresh"
	tk = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refresh, err = tk.SignedString([]byte(os.Getenv("BUILDRONE_WEBSECRET")))
	if err != nil {
		return "", "", err
	}
	return token, refresh, nil
}

func jwtBuildTokenWrapper(secret string) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}
}

func jwtWebToken(token *jwt.Token) (interface{}, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method %v", token.Header["alg"])
	}
	return []byte(os.Getenv("BUILDRONE_WEBSECRET")), nil
}

type getTokenDTO struct {
	Token   string `json:"token" example:"kjsdklsfdkljfsjsdfklsdfkldsfjdfskjsdfjklsdf"` // API token for use with everything else.
	Refresh string `json:"refresh"`
}

func abort(status int, message string, gc *gin.Context) {
	gc.AbortWithStatusJSON(status, map[string]string{"error": message})
}

func end(status int, message string, gc *gin.Context) {
	gc.JSON(status, map[string]string{"error": message})
}

func (app *appContext) validateBuildToken(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	if _, ok := app.storage[namespace+"/"+name]; !ok {
		abort(401, fmt.Sprintf("Repo not found: %s/%s", namespace, name), gc)
		return
	}
	header := strings.SplitN(gc.Request.Header.Get("Authorization"), "Bearer ", 2)
	if len(header) < 2 {
		log.Printf("%s/%s: Header too short", namespace, name)
		abort(401, "Unauthorized", gc)
		return
	}
	encoded := header[1]
	auth, _ := base64.StdEncoding.DecodeString(encoded)
	repo := app.storage[namespace+"/"+name]
	secret := repo.Secret
	token, err := jwt.Parse(string(auth), jwtBuildTokenWrapper(secret))
	if err != nil {
		log.Printf("%s/%s getBuildToken: Error parsing JWT: %s", namespace, name, err)
		abort(401, "Unauthorized", gc)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	expiryUnix, err := strconv.ParseInt(claims["exp"].(string), 10, 64)
	if err != nil {
		log.Printf("%s/%s getBuildToken: Error parsing expiry: %s", namespace, name, err)
		abort(401, "Unauthorized", gc)
		return
	}
	expiry := time.Unix(expiryUnix, 0)
	if !(ok && token.Valid && claims["type"].(string) == "bearer" && expiry.After(time.Now())) {
		log.Printf("%s/%s getBuildToken: Auth denied: Ok %t, Type %s, Expiry %s", namespace, name, ok, claims["type"].(string), expiry)
		abort(401, "Unauthorized", gc)
		return
	}
	if claims["namespace"].(string) != repo.Namespace || claims["repo"].(string) != repo.Name {
		log.Printf("%s/%s getBuildToken: Auth denied: Namespace or Repo invalid", namespace, name)
		abort(401, "Unauthorized", gc)
		return
	}
	gc.Next()
}

func (app *appContext) validateWebToken(gc *gin.Context) {
	header := strings.SplitN(gc.Request.Header.Get("Authorization"), "Bearer ", 2)
	encoded := header[1]
	auth, _ := base64.StdEncoding.DecodeString(encoded)
	token, err := jwt.Parse(string(auth), jwtWebToken)
	if err != nil {
		abort(401, "Unauthorized", gc)
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	expiryUnix, err := strconv.ParseInt(claims["exp"].(string), 10, 64)
	if err != nil {
		abort(401, "Unauthorized", gc)
		return
	}
	expiry := time.Unix(expiryUnix, 0)
	if !(ok && token.Valid && claims["type"].(string) == "bearer" && expiry.After(time.Now())) {
		abort(401, "Unauthorized", gc)
		return
	}
	gc.Next()
}

func (app *appContext) getBuildToken(gc *gin.Context) {
	namespace := gc.Param("namespace")
	name := gc.Param("name")
	if _, ok := app.storage[namespace+"/"+name]; !ok {
		end(401, fmt.Sprintf("Repo not found: %s/%s", namespace, name), gc)
		return
	}
	header := strings.SplitN(gc.Request.Header.Get("Authorization"), "Bearer ", 2)
	encoded := header[1]
	auth, _ := base64.StdEncoding.DecodeString(encoded)
	repo := app.storage[namespace+"/"+name]
	refresh, err := jwt.Parse(string(auth), jwtBuildTokenWrapper(repo.Secret))
	if err != nil {
		log.Printf("%s/%s getBuildToken: Error parsing JWT: %s", namespace, name, err)
		end(401, "Unauthorized", gc)
		return
	}
	claims, ok := refresh.Claims.(jwt.MapClaims)
	expiryUnix, err := strconv.ParseInt(claims["exp"].(string), 10, 64)
	if err != nil {
		log.Printf("%s/%s getBuildToken: Error parsing expiry: %s", namespace, name, err)
		end(401, "Unauthorized", gc)
		return
	}
	expiry := time.Unix(expiryUnix, 0)
	if !(ok && refresh.Valid && claims["type"].(string) == "refresh" && expiry.After(time.Now())) {
		log.Printf("%s/%s getBuildToken: Auth denied: Ok %t, Type %s, Expiry %s", namespace, name, ok, claims["type"].(string), expiry)
		end(401, "Unauthorized", gc)
		return
	}
	if claims["namespace"].(string) != repo.Namespace || claims["repo"].(string) != repo.Name {
		log.Printf("%s/%s getBuildToken: Auth denied: Namespace or Repo invalid", namespace, name)
		end(401, "Unauthorized", gc)
		return
	}
	token, newRefresh, err := newBuildToken(namespace, name, repo.Secret)
	if err != nil {
		end(500, "Couldn't generate token", gc)
		return
	}
	gc.JSON(200, getTokenDTO{Token: token, Refresh: newRefresh})
}

func (app *appContext) getWebToken(gc *gin.Context) {
	var token, newRefresh string
	header := strings.SplitN(gc.Request.Header.Get("Authorization"), "Basic ", 2)
	encoded := header[1]
	auth, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		end(401, "Unauthorized", gc)
		return
	}
	credentials := strings.SplitN(string(auth), ":", 2)
	if len(credentials) >= 2 && credentials[0] != "" && credentials[1] != "" {
		log.Printf("Auth requested using username/password")
		username := app.config.Section("").Key("username").String()
		password := app.config.Section("").Key("password_hash").String()
		if !(credentials[0] == username && checkHash(credentials[1], password)) {
			end(401, "Unauthorized", gc)
			return
		}
	} else {
		log.Printf("Auth requested using refresh token")
		cookie, err := gc.Cookie("refresh")
		if err != nil {
			end(401, "Unauthorized", gc)
			return
		}
		refresh, err := jwt.Parse(string(cookie), jwtWebToken)
		if err != nil {
			end(401, "Unauthorized", gc)
			return
		}
		claims, ok := refresh.Claims.(jwt.MapClaims)
		expiryUnix, err := strconv.ParseInt(claims["exp"].(string), 10, 64)
		if err != nil {
			end(401, "Unauthorized", gc)
			return
		}
		expiry := time.Unix(expiryUnix, 0)
		if !(ok && refresh.Valid && claims["type"].(string) == "refresh" && expiry.After(time.Now())) {
			end(401, "Unauthorized", gc)
			return
		}
	}
	token, newRefresh, err = newWebTokens()
	if err != nil {
		end(401, "Unauthorized", gc)
		return
	}
	gc.SetCookie("refresh", newRefresh, (3600 * 24), "/", gc.Request.URL.Hostname(), true, true)
	gc.JSON(200, getTokenDTO{Token: token})
}
