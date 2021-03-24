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
	"strconv"
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
	ReleasesURL:   "https://api.github.com/repos/{ower}/{repo}/releases",
}

// endpoint contains the different authentication urls for a given service
type endpoint struct {
	AuthURL       string
	DeviceAuthURL string
	TokenURL      string
	ReleasesURL   string
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

type releases []release
type release struct {
	TagName         *string `json:"tag_name,omitempty"`
	TargetCommitish *string `json:"target_commitish,omitempty"`
	Name            *string `json:"name,omitempty"`
	Body            *string `json:"body,omitempty"`
	Draft           *bool   `json:"draft,omitempty"`
	Prerelease      *bool   `json:"prerelease,omitempty"`

	ID          *int     `json:"id,omitempty"`
	CreatedAt   *string  `json:"created_at,omitempty"`
	PublishedAt *string  `json:"published_at,omitempty"`
	URL         *string  `json:"url,omitempty"`
	HTMLURL     *string  `json:"html_url,omitempty"`
	AssetsURL   *string  `json:"asset_url,omitempty"`
	Assets      []*asset `json:"assets,omitempty"`
	UploadURL   *string  `json:"upload_url,omitempty"`
	ZipballURL  *string  `json:"zipball_url,omitempty"`
	TarballURL  *string  `json:"tarball_url,omitempty"`
	Author      *user    `json:"author,omitempty"`
	NodeID      *string  `json:"node_id,omitempty"`
	raw         map[string]interface{}
}

type asset struct {
	ID                 *int64  `json:"id,omitempty"`
	URL                *string `json:"url,omitempty"`
	Name               *string `json:"name,omitempty"`
	Label              *string `json:"label,omitempty"`
	State              *string `json:"state,omitempty"`
	ContentType        *string `json:"content_type,omitempty"`
	Size               *int    `json:"size,omitempty"`
	DownloadCount      *int    `json:"download_count,omitempty"`
	CreatedAt          *string `json:"created_at,omitempty"`
	UpdatedAt          *string `json:"updated_at,omitempty"`
	BrowserDownloadURL *string `json:"browser_download_url,omitempty"`
	Uploader           *string `json:"uploader,omitempty"`
	NodeID             *string `json:"node_id,omitempty"`
	raw                map[string]interface{}
}

type user interface {
}

// newGetRequest creates an http.Request using the provided URL
// and sets the Content-Type and Accept headers to values we can work with
func newGetRequest(url string, params url.Values) (*http.Request, error) {
	r, err := http.NewRequest("GET", url, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	// content is submitted as x-www-form-urlencoded; accepted back as JSON
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")

	return r, nil
}

// newPostRequest creates an http.Request using the provided URL and parameters
// and sets the Content-Type and Accept headers to values we can work with
func newPostRequest(url string, params url.Values, headers ...map[string]string) (*http.Request, error) {
	r, err := http.NewRequest("POST", url, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}

	// content is submitted as x-www-form-urlencoded; accepted back as JSON
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// TODO: test Github recommended Accept header
	// Github recommended Accept header is: application/vnd.github.v3+json
	// r.Header.Set("Accept", "application/vnd.github.v3+json")
	r.Header.Set("Accept", "application/json")

	// set any additional headers passed into the function
	// this seems like an ugly way to do this
	// a slice of maps? yuck
	if len(headers) > 0 {
		for _, h := range headers {
			for k, v := range h {
				r.Header.Set(k, v)
			}
		}
	}

	return r, nil
}

// getAccessToken calls into the userAuthURL to check and see if the user has authorized
// this device to act on their behalf, and returns a response
func getAccessToken(req *http.Request) (*UserAuth, bool, error) {

	// make the request
	body, err := makeHTTPRequest(req)
	if err != nil {
		return nil, false, err
	}

	// unmarshal the respone
	var auth = &UserAuth{}
	if err = json.Unmarshal(body, &auth); err != nil {
		return nil, false, err
	}

	// unmarshal the raw data
	if err = json.Unmarshal(body, &auth.raw); err != nil {
		return nil, false, err
	}

	switch e := auth.raw["error"]; e {
	case "authorization_pending":
		return auth, false, nil
	case "slow_down":
		return auth, false, fmt.Errorf("%v", e)
	case "expired_token":
		return auth, false, fmt.Errorf("%v", e)
	case "unsupported_grant_type":
		return auth, false, fmt.Errorf("%v", e)
	case "incorrect_client_credentials":
		return auth, false, fmt.Errorf("%v", e)
	case "incorrect_device_code":
		return auth, false, fmt.Errorf("%v", e)
	case "access_denied":
		return auth, false, fmt.Errorf("%v", e)
	}

	return auth, true, nil

}

// make HTTPRequest takes an http.Request, executes the request, checks for a 200
// response, and reads the response body to a byte slice
func makeHTTPRequest(req *http.Request) ([]byte, error) {

	fmt.Println("DEBUG: beginning 'makeHTTPRequest'")

	// create a context and execute the http request
	r, err := ctxhttp.Do(context.TODO(), nil, req)
	if err != nil {
		return nil, err
	}

	fmt.Println("DEBUG: ctxhttp.Do called in 'makeHTTPRequest'")

	// check to see if the initial device flow request succeded
	// 422 is an unprocessable entity, anything else is unknown
	if code := r.StatusCode; code == 404 {
		return nil, fmt.Errorf("failed to authorize device: %s", r.Status)
	}
	if code := r.StatusCode; code != 200 {
		return nil, fmt.Errorf(r.Status)
	}

	fmt.Println("DEBUG: r.StatusCode processed in 'makeHTTPRequest'")

	fmt.Println("DEBUG: read response in 'makeHTTPRequest'")
	// read the body of the returned request
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
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

	for {
		select {
		case <-timeout:
			return nil, errors.New("timeout reached")
		case <-ticker:
			// create an http.Request
			req, err := newPostRequest(userAuthURL, params)
			if err != nil {
				return nil, err
			}
			auth, ok, err := getAccessToken(req)
			if ok {
				return auth, nil
			}
			if err != nil {
				if err.Error() == "slow_down" {
					fmt.Printf("slow down; adding %v seconds to interval\n", auth.raw["interval"])
					interval = int(auth.raw["interval"].(float64)) + 1
					ticker = time.Tick(time.Duration(interval)*time.Second + 1)
					time.Sleep(time.Duration(interval))
				} else {
					return auth, err
				}
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
	if err = json.Unmarshal(body, &auth); err != nil {
		return nil, err
	}

	// unmarshal the raw data
	if err = json.Unmarshal(body, &auth.raw); err != nil {
		return nil, err
	}

	return auth, err
}

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

// getReleases retrieves a slice of releases from the gitURL
func getReleases(gURL *gitURL) (*releases, error) {
	var releasesList releases
	// TEMP RELEASES URL HERE; LEARN TO TEMPLATE AND USE githubEndpoint.ReleasesURL
	releasesURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", gURL.organization, gURL.repository)

	req, err := newGetRequest(releasesURL, url.Values{})
	if err != nil {
		return &releasesList, err
	}

	body, err := makeHTTPRequest(req)
	if err != nil {
		return &releasesList, err
	}

	if err = json.Unmarshal(body, &releasesList); err != nil {
		return &releasesList, err
	}

	// unmarshal the raw data
	if err = json.Unmarshal(body, &releasesList); err != nil {
		return nil, err
	}

	return &releasesList, nil
}

// createRelease accepts a release name, description, commit value, tag name, target_commitish
func createRelease(auth *UserAuth, gURL *gitURL, tag, tagMessage, commitish string, draft, prerelease bool) (*release, error) {
	// TEMP RELEASES URL HERE; LEARN TO TEMPLATE AND USE githubEndpoint.ReleasesURL
	releasesURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", gURL.organization, gURL.repository)

	headers := make(map[string]string)

	params := url.Values{}
	params.Add("tag_name", tag)
	params.Add("name", "DEBUG TEST 1")
	params.Add("body", tagMessage)
	params.Add("draft", strconv.FormatBool(draft))
	params.Add("prerelease", strconv.FormatBool(prerelease))

	fmt.Printf("+%v\n", params)

	headers["Authorization"] = fmt.Sprintf("%s %s", auth.TokenType, auth.AccessToken)
	req, err := newPostRequest(releasesURL, url.Values{}, headers)
	if err != nil {
		return nil, err
	}

	fmt.Println("DEBUG: returned successfully from 'newPostRequest'")

	// make the request
	body, err := makeHTTPRequest(req)
	if err != nil {
		return nil, err
	}

	fmt.Println("DEBUG: returned successfully from 'makeHTTPRequest'")

	var newRelease release
	if err := json.Unmarshal(body, &newRelease); err != nil {
		return nil, err
	}
	// unmarshal the raw data
	if err = json.Unmarshal(body, &newRelease.raw); err != nil {
		return nil, err
	}

	fmt.Println("DEBUG: returned successfully from JSON unmarshalling")

	return &newRelease, nil
}
