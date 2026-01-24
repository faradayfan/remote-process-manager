package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type API struct {
	client    *http.Client
	baseURL   string
	apiPrefix string
	apiKey    string // NEW
}

func NewAPI(client *http.Client, baseURL, apiPrefix, apiKey string) *API {
	return &API{
		client:    client,
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiPrefix: apiPrefix,
		apiKey:    apiKey, // NEW
	}
}

func (a *API) url(path string) string {
	return a.baseURL + a.apiPrefix + path
}

// NEW: setAuth adds authorization header if API key is configured
func (a *API) setAuth(req *http.Request) {
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}
}

func (a *API) PrintGET(path string) error {
	body, status, err := a.GET(path)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", prettyJSON(body))
	if status >= 400 {
		return fmt.Errorf("request failed: HTTP %d", status)
	}
	return nil
}

func (a *API) PrintPOST(path string, payload any) error {
	body, status, err := a.POST(path, payload)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", prettyJSON(body))
	if status >= 400 {
		return fmt.Errorf("request failed: HTTP %d", status)
	}
	return nil
}

func (a *API) GET(path string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", a.url(path), nil)
	if err != nil {
		return nil, 0, err
	}

	a.setAuth(req) // NEW

	res, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	return b, res.StatusCode, nil
}

func (a *API) POST(path string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest("POST", a.url(path), body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	a.setAuth(req) // NEW

	res, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	return respBody, res.StatusCode, nil
}
