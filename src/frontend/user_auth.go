package main

import (
	"./api"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/net/context"
	"log"
	"net/http"
)

type googleOAuth2V3Resp struct {
	Email   string `json:"email"`
	Locale  string `json:"locale"`
	Picture string `json:"picture"`
}

// Decided if the user is allowed to do write operations.
func (fe *FeServiceImpl) isWriteAuthorized(ctx context.Context, auth *api.UserAuth) (bool, error) {
	ctx = TraceStart(ctx, "/frontend._.isWriteAuthorized")
	defer TraceEnd(ctx, fe.ServerCred)

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
	if m.Email != "xanxys@gmail.com" {
		log.Printf("Logged in as non-authorized user %#v", m)
		return false, nil
	}
	return true, nil
}
