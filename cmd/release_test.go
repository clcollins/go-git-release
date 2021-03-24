package cmd

import (
	. "github.com/stretchr/testify/assert"

	"fmt"
	"net/url"
	"strconv"
	"testing"
)

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
