package cmd

import (
	. "github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"

	"fmt"
	"net/url"
	"strconv"
	"testing"
)

// TestNewGetRequest calls newGetRequest,
// and checks the resulting request contains the expected payload
func TestNewGetRequest(t *testing.T) {
	endPointURL := "https://api.example.org/api/testendpoint"
	expectedMethod := "GET"

	expectedURL := &url.URL{
		Scheme: "https",
		Host:   "api.example.org",
		Path:   "/api/testendpoint",
	}

	params := url.Values{}

	req, err := newGetRequest(endPointURL, params)
	Nil(t, err)

	Equal(t, expectedMethod, req.Method)
	Equal(t, expectedURL, req.URL)
}

// TestNewPostRequest calls newPostRequest, and checks the resulting request
func TestNewPostRequest(t *testing.T) {

	// Input Values
	endPointURL := "https://api.example.org/api/testendpoint"
	params := url.Values{}
	params.Add("tag_name", "v1.0")
	params.Add("name", "Version 1.0 Release")
	params.Add("draft", strconv.FormatBool(false))
	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("bearer BEAR!RUN!")

	// Expected Results
	expectedMethod := "POST"

	expectedHeader := map[string][]string{
		"Authorization": {"bearer BEAR!RUN!"},
		"Content-Type":  {"application/x-www-form-urlencoded"},
		"Accept":        {"application/json"},
	}

	expectedURL := &url.URL{
		Scheme: "https",
		Host:   "api.example.org",
		Path:   "/api/testendpoint",
	}

	req, err := newPostRequest(endPointURL, params, headers)
	Nil(t, err)

	Equal(t, expectedMethod, req.Method)
	Equal(t, fmt.Sprint(expectedHeader), fmt.Sprint(req.Header))
	Equal(t, expectedURL, req.URL)

	// Parse the form data from the returned request
	// Must be done to access it
	parseErr := req.ParseForm()
	Nil(t, parseErr)
	Equal(t, params, req.PostForm)

}

// TestGetAccessToken mocks an HTTP Request for an auth token,
// and checks for the correct error handling
func TestGetAccessToken(t *testing.T) {
	// Test cases of errors responded by Github:
	// authorization_pending
	// slow_down
	// expired_token
	// unsuppored_grant_type
	// incorrect_client_credentials
	// incorrect_device_code
	// access_denied
	return
}

// TestMakeHTTPRequest mocks calling out an http requests to a real server
// and checks the right request is being passed and handled
func TestMakeHTTPRequest(t *testing.T) {
	defer gock.Off()

	gock.New("https://api.github.com").
		MatchHeader("Content-Type", "^application/x-www-form-urlencoded$").
		MatchHeader("Accept", "^application/json$").
		Reply(200).
		JSON(map[string]string{"foo": "bar"})

	endPointURL := "https://api.github.com/api/testendpoint"
	params := url.Values{}

	req, err := newGetRequest(endPointURL, params)

	_, err = makeHTTPRequest(req)
	Nil(t, err)

	// Also test 404, 422, and 200 responses
	return
}

// TestPollForAccessToken mocks responses to requests asking for access tokens
// (eg: checking that the user has approved the device auth), and the handling
// of errors
func TestPollForAccessToken(t *testing.T) {
	return
}

// TestRequestDeviceAndUserCodes tests calls to Github to request device
// auth codes
func TestRequestDeviceAndUserCodes(t *testing.T) {
	return
}

// TestGetReleases
func TestGetReleases(t *testing.T) {
	return
}

// TestCreateRelease
func TestCreateRelease(t *testing.T) {
	return
}
