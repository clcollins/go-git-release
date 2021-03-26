package cmd

import (
	"encoding/json"

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
	headers["Authorization"] = "bearer abc123"

	// Expected Results
	expectedMethod := "POST"

	expectedHeader := map[string][]string{
		"Authorization": {"bearer abc123"},
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
}

// TestMakeHTTPRequest mocks calling out an http requests to a real server
// and checks the right request is being passed and handled
func TestMakeHTTPRequest(t *testing.T) {
	defer gock.Off()

	var tagName string = "v1.0"
	var targetCommitish string = "968c07f04cddcabd95cbdc42f1ad0b99854395e3"
	var name string = "Version 1.0"

	httpTests := []struct {
		name            string
		reply           int
		expected        string
		inputJSON       map[string]string
		expectedContent release
	}{
		{
			name:     "Test 200 response 01",
			reply:    200,
			expected: "",
			inputJSON: map[string]string{
				"tag_name":         "v1.0",
				"target_commitish": "968c07f04cddcabd95cbdc42f1ad0b99854395e3",
				"name":             "Version 1.0",
			},
			expectedContent: release{
				TagName:         &tagName,
				TargetCommitish: &targetCommitish,
				Name:            &name,
			},
		},
		{
			name:     "Test 404 response",
			reply:    404,
			expected: "404 Not Found",
		},
		{
			name:     "Test 422 response",
			reply:    422,
			expected: "422 Unprocessable Entity",
		},
	}

	gockMatchURL := "https://api.github.com/api/testendpoint"
	endPointURL := "https://api.github.com/api/testendpoint"

	params := url.Values{}

	for _, testSpec := range httpTests {
		t.Run(
			testSpec.name,
			func(t *testing.T) {
				gock.New(gockMatchURL).
					MatchHeader("Content-Type", "^application/x-www-form-urlencoded$").
					MatchHeader("Accept", "^application/json$").
					Reply(testSpec.reply).
					JSON(testSpec.inputJSON)

				req, err := newGetRequest(endPointURL, params)
				Nil(t, err)

				body, err := makeHTTPRequest(req)
				if testSpec.expected != "" {
					Error(t, err)
					Equal(t, testSpec.expected, err.Error())
					False(t, json.Valid(body))
				}

				if testSpec.expected == "" {
					Nil(t, err)
					True(t, json.Valid(body))
					var b = &release{}
					err = json.Unmarshal(body, b)
					fmt.Println(b)
					Nil(t, err)
					Equal(t, testSpec.expectedContent.TagName, b.TagName)
					Equal(t, testSpec.expectedContent.TargetCommitish, b.TargetCommitish)
					Equal(t, testSpec.expectedContent.Name, b.Name)

				}

			},
		)
	}
}

// TestPollForAccessToken mocks responses to requests asking for access tokens
// (eg: checking that the user has approved the device auth), and the handling
// of errors
func TestPollForAccessToken(t *testing.T) {
}

// TestRequestDeviceAndUserCodes tests calls to Github to request device
// auth codes
func TestRequestDeviceAndUserCodes(t *testing.T) {
}

// TestGetReleases
func TestGetReleases(t *testing.T) {
}

// TestCreateRelease
func TestCreateRelease(t *testing.T) {
}
