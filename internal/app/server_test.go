package app_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"testing"

	"buildingops/internal/app"
	"buildingops/internal/audit"
	"buildingops/internal/auth"
	"buildingops/internal/operations"
	"net/http/httptest"
)

type statusResponse struct {
	Assets []operations.AssetStatus `json:"assets"`
}

type auditResponse struct {
	Entries []audit.Entry `json:"entries"`
}

func TestAuthenticatedOperationsFlow(t *testing.T) {
	server := httptest.NewServer(app.NewServer(app.Options{}))
	t.Cleanup(server.Close)
	client := newClient(t)

	response := get(t, client, server.URL+"/api/status")
	wantStatus(t, response, http.StatusUnauthorized)
	closeBody(response)

	response = get(t, client, server.URL+"/")
	wantStatus(t, response, http.StatusOK)
	wantBody(t, response, "Sign in to continue")

	response = login(t, client, server.URL, "ops.admin", "facility-123")
	wantStatus(t, response, http.StatusSeeOther)
	closeBody(response)

	response = get(t, client, server.URL+"/")
	wantStatus(t, response, http.StatusOK)
	body := readBody(t, response)
	for _, text := range []string{"Operations overview", "Service status", "Recent access activity"} {
		if !strings.Contains(body, text) {
			t.Fatalf("authenticated page missing %q", text)
		}
	}

	response = get(t, client, server.URL+"/api/status")
	wantStatus(t, response, http.StatusOK)
	var statuses statusResponse
	decodeJSON(t, response, &statuses)
	if len(statuses.Assets) != 3 {
		t.Fatalf("asset count = %d, want 3", len(statuses.Assets))
	}
	wantKinds := []string{"elevator", "hvac", "access"}
	for index, kind := range wantKinds {
		if statuses.Assets[index].Kind != kind {
			t.Fatalf("asset %d kind = %q, want %q", index, statuses.Assets[index].Kind, kind)
		}
	}

	response = postForm(t, client, server.URL+"/logout", url.Values{})
	wantStatus(t, response, http.StatusSeeOther)
	closeBody(response)
	response = get(t, client, server.URL+"/api/audit")
	wantStatus(t, response, http.StatusUnauthorized)
	closeBody(response)

	response = login(t, client, server.URL, "ops.admin", "facility-123")
	wantStatus(t, response, http.StatusSeeOther)
	closeBody(response)
	response = get(t, client, server.URL+"/api/audit")
	wantStatus(t, response, http.StatusOK)
	var history auditResponse
	decodeJSON(t, response, &history)
	wantActions := []string{"login", "logout", "login"}
	if len(history.Entries) != len(wantActions) {
		t.Fatalf("audit entry count = %d, want %d", len(history.Entries), len(wantActions))
	}
	for index, action := range wantActions {
		if history.Entries[index].Action != action {
			t.Fatalf("audit entry %d action = %q, want %q", index, history.Entries[index].Action, action)
		}
	}
}

func TestRepeatedFailuresLockAccount(t *testing.T) {
	server := httptest.NewServer(app.NewServer(app.Options{}))
	t.Cleanup(server.Close)
	client := newClient(t)

	response := login(t, client, server.URL, "ops.admin", "incorrect-one")
	wantStatus(t, response, http.StatusUnauthorized)
	closeBody(response)
	response = login(t, client, server.URL, "ops.admin", "incorrect-two")
	wantStatus(t, response, http.StatusLocked)
	closeBody(response)
	response = login(t, client, server.URL, "ops.admin", "facility-123")
	wantStatus(t, response, http.StatusLocked)
	closeBody(response)
}

func TestConcurrentFailuresLockAccount(t *testing.T) {
	barrier := auth.NewBarrier(2)
	server := httptest.NewServer(app.NewServer(app.Options{FailureBarrier: barrier}))
	t.Cleanup(server.Close)

	statuses := make(chan int, 2)
	errors := make(chan error, 2)
	var group sync.WaitGroup
	for _, password := range []string{"incorrect-one", "incorrect-two"} {
		group.Add(1)
		go func(password string) {
			defer group.Done()
			client, err := clientWithoutRedirects()
			if err != nil {
				errors <- err
				return
			}
			response, err := client.PostForm(server.URL+"/login", url.Values{
				"username": {"ops.admin"},
				"password": {password},
			})
			if err != nil {
				errors <- err
				return
			}
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			statuses <- response.StatusCode
		}(password)
	}
	group.Wait()
	close(errors)
	close(statuses)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	for status := range statuses {
		if status != http.StatusUnauthorized && status != http.StatusLocked {
			t.Fatalf("concurrent failure status = %d, want 401 or 423", status)
		}
	}

	client := newClient(t)
	response := login(t, client, server.URL, "ops.admin", "facility-123")
	wantStatus(t, response, http.StatusLocked)
	closeBody(response)
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	client, err := clientWithoutRedirects()
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func clientWithoutRedirects() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

func login(t *testing.T, client *http.Client, baseURL, username, password string) *http.Response {
	t.Helper()
	return postForm(t, client, baseURL+"/login", url.Values{
		"username": {username},
		"password": {password},
	})
}

func postForm(t *testing.T, client *http.Client, target string, values url.Values) *http.Response {
	t.Helper()
	response, err := client.PostForm(target, values)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func get(t *testing.T, client *http.Client, target string) *http.Response {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func wantStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, want, body)
	}
}

func closeBody(response *http.Response) {
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
}

func wantBody(t *testing.T, response *http.Response, text string) {
	t.Helper()
	body := readBody(t, response)
	if !strings.Contains(body, text) {
		t.Fatalf("response body missing %q", text)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func decodeJSON(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
