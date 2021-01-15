package cmd

/*
Copyright © 2020 Red Hat Inc.

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

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"
)

// TODO: POST /repos/{owner}/{repo}/releases
// accept string header; "application/vnd.github.v3+json"
// tag_name string (required)
// target_commitish string - the commitish value for the tag; unused if tag exists
// name string name of the release
// body string text of the tag
// draft bool create a draft release
// prerelease bool create a prerelease

// TODO: POST upload_url
// upload_url returned by ^ is the endpoint to upload assets:
//  "upload_url": "https://uploads.github.com/repos/octocat/Hello-World/releases/1/assets{?name,label}",
// use Content-Type to provide the media type of the asset.
// must be in RAW binary form, not json, as the request body
// Upstream errors return 502 Bad Gateway, may leave empty asset with state `starter` - should be deleted
// must delete asset of same name before reupload

// TODO: GET /repos/{owner}/{repo}/releases
// TODO: GET /repos/{owner}/{repo}/releases/assets/{asset_id}
// Package cmd is the main cobra command package

const githubDeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

const (
	errAuthorizationPending       = "pending"
	errSlowDown                   = "slow_down"
	errExpiredToken               = "expired_token"
	errUnsupportedGrantType       = "unsupported_grant_type"
	errIncorrectClientCredentials = "incorrect_client_credentials"
	errIncorrectDeviceCode        = "incorrect_device_code"
	errAccessDenied               = "access_denied"
)

// githubEndpoint is an endpoint representation for GitHub API authentication
var githubEndpoint = endpoint{
	AuthURL:       "",
	DeviceAuthURL: "https://github.com/login/device/code",
	TokenURL:      "https://github.com/login/oauth/access_token",
}

// endpoint contains the different authentication urls for a given service
type endpoint struct {
	AuthURL       string
	DeviceAuthURL string
	TokenURL      string
}

// DeviceAuth contains the response from an OAuth2 device flow auth request
type DeviceAuth struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri,verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	raw             map[string]interface{}
}

// UserAuth contains the response from an OAuth2 device flow authentication polling request
type UserAuth struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	raw         map[string]interface{}
}

// newPostRequest creates an http.Request using the provided URL and parameters
// and sets the Content-Type and Accept headers to values we can work with
func newPostRequest(url string, params url.Values) (*http.Request, error) {
	r, err := http.NewRequest("POST", url, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	// content is submitted as x-www-form-urlencoded; accepted back as JSON
	r.Header.Set("Contet-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")

	return r, nil
}

// retrieveAccessToken calls into the userAuthURL to check and see if the user has authorized
// this device to act on their behalf, and returns a response
func retrieveAccessToken(req *http.Request) (*UserAuth, bool, error) {

	// make the request
	body, err := makeHTTPRequest(req)
	if err != nil {
		return nil, false, err
	}

	// unmarshal the respone
	var auth = &UserAuth{}
	err = json.Unmarshal(body, &auth)
	if err != nil {
		if verbose {
			fmt.Println("error unmarshallng JSON response")
		}
		return nil, false, err
	}

	// unmarshal the raw data
	err = json.Unmarshal(body, &auth.raw)
	if err != nil {
		if verbose {
			fmt.Println("error unmarshalling raw JSON response")
		}
		return nil, false, err
	}

	if reqErr, ok := auth.raw["error"]; ok {
		switch reqErr {
		case "authorization_pending":
			return auth, false, nil
		default:
			return auth, false, fmt.Errorf("%v", reqErr)
		}
	}

	return auth, true, err
}

// make HTTPRequest takes an http.Request, executes the request, checks for a 200
// response, and reads the response body to a byte slice
func makeHTTPRequest(req *http.Request) ([]byte, error) {
	// create a context and execute the http request
	r, err := ctxhttp.Do(context.TODO(), nil, req)
	if err != nil {
		if verbose {
			fmt.Println("error executing http request")
		}
		return nil, err
	}

	// check to see if the initial device flow request succeded
	if code := r.StatusCode; code != 200 {
		if verbose {
			fmt.Printf("error bad response code: %d\n", code)
		}
		return nil, fmt.Errorf("failed device auth initiation: %s", r.Status)
	}

	// read the body of the returned request
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		if verbose {
			fmt.Println("error ready response body")
		}
		return nil, err
	}

	return body, err
}

// pollForAccessToken checks each specified interval for a response containing an accessToken, until the time limit expires
func pollForAccessToken(userAuthURL, clientID, deviceCode, grantType string, expiresIn, interval int) (*UserAuth, error) {

	timeout := time.After(time.Duration(expiresIn) * time.Second)
	// allowed to poll every `interval`, so just add a second to not be greedy
	ticker := time.Tick(time.Duration(interval)*time.Second + 1)

	// set input parameters
	params := url.Values{}
	params.Add("client_id", clientID)
	params.Add("device_code", deviceCode)
	params.Add("grant_type", grantType)

	// create an http.Request
	req, err := newPostRequest(userAuthURL, params)
	if err != nil {
		return nil, err
	}

	for {
		select {
		case <-timeout:
			return nil, errors.New("timeout reached")
		case <-ticker:
			auth, ok, err := retrieveAccessToken(req)
			if err != nil {
				fmt.Println(err)
				if err.Error() == "slow_down" {
					if verbose {
						fmt.Printf("Warning: Slow Down\n Slow to: %+vs\n", auth.raw["interval"])
					}
					interval = int(auth.raw["interval"].(float64)) + 1
					ticker = time.Tick(time.Duration(interval)*time.Second + 1)
				} else {
					return nil, err
				}
			} else if ok {
				return auth, nil
			}
		}
	}

}

// Call the device login url
func requestDeviceAndUserCodes(deviceAuthURL, clientID, scope string) (*DeviceAuth, error) {

	// set input parameters for client_id and scope
	params := url.Values{}
	params.Add("client_id", clientID)

	// scope is optional
	if scope != "" {
		params.Set("scope", scope)
	}

	// create the HTTP request for the device authentication
	req, err := newPostRequest(deviceAuthURL, params)
	if err != nil {
		return nil, err
	}

	// make the request
	body, err := makeHTTPRequest(req)
	if err != nil {
		return nil, err
	}

	// unmarshal the response
	var auth = &DeviceAuth{}
	err = json.Unmarshal(body, &auth)
	if err != nil {
		if verbose {
			fmt.Println("error unmarshallng JSON response")
		}
		return nil, err
	}

	// unmarshal the raw data
	err = json.Unmarshal(body, &auth.raw)
	if err != nil {
		if verbose {
			fmt.Println("error unmarshalling raw JSON response")
		}
		return nil, err
	}

	return auth, err
}

// func main() {
// 	scope := "user:email"
// 	authResponse, err := requestDeviceAndUserCodes(githubEndpoint.DeviceAuthURL, clientID, scope)
// 	if err != nil {
// 		log.Fatalf(err.Error())
// 	}
// 	fmt.Printf(authResponse.DeviceCode)
//
// }

// PROMPT USER TO ENTER USER CODE TO VERIFICATION URI

// https://github.com/login/oauth/access_token client_id=REDACTED device_code=REDACTED grant_type=urn:ietf:params:oauth:grant-type:device_code
// {
//  "access_token": REDACTED,
//  "token_type": "bearer",
//  "scope": "user"
// }

// GET A THING
// --auth-type=token --auth-"bearer:REDACTED" https://github.com/user/emails

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Failed to automatically open browser. Please manually visit %s in a browser window.", url)
	}

}