package main

import (
	"fmt"
	"net/http"
)

type playerAccessChecker interface{ CheckAccess(string) error }
type apiPlayerAccessChecker struct {
	url    string
	client *http.Client
	secret string
}

func (checker *apiPlayerAccessChecker) CheckAccess(userID string) error {
	req, err := http.NewRequest(http.MethodGet, checker.url+"/internal/users/"+userID+"/access", nil)
	if err != nil {
		return err
	}
	setInternalSecret(req, checker.secret)
	response, err := checker.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("player access returned status %d", response.StatusCode)
	}
	return nil
}
