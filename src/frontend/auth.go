package main

import (
	"./api"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
)

type googleOAuth2V3Resp struct {
	email string
}

// Decided if the user is allowed to do write operations.
func (fe *FeServiceImpl) isWriteAuthorized(auth *api.UserAuth) (bool, error) {
	log.Printf("Validating authentication token")
	resp, err := http.Get(fmt.Sprintf("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=%s", auth.IdToken))
	if err != nil {
		return false, errors.New("Auth server failed")
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var m googleOAuth2V3Resp
	err = decoder.Decode(&m)
	if err != nil {
		return false, err
	}
	if m.email != "xanxys@gmail.com" {
		log.Printf("Logged in as non-authorized user %s", m.email)
		return false, nil
	}
	return true, nil
}
