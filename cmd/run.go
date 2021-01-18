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

// Package cmd is the main cobra command package
package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func createTempDir() (string, error) {
	// find some way to specify this by project?
	prefix := "ggt-"

	tempDir, err := ioutil.TempDir(os.TempDir(), prefix)

	if err != nil {
		return "", err
	}

	return tempDir, nil

}

func createTempFile() (*os.File, error) {
	// find some way to specify this by project?
	prefix := "ggt-"

	tempFile, err := ioutil.TempFile(os.TempDir(), prefix)

	if err != nil {
		return nil, err
	}

	return tempFile, nil

}

// confirm prompts the user for yes or no, with a message from the provided string
// Immediately returns true (yes) if the "force" flag is set
func confirm(s string) bool {
	// If the force flag is set, assume true
	if force {
		return true
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", s)

		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}

// cloneRepo clones the provided git repository into the provided directory using the SSH Agent "git" identity
func cloneRepo(url, dir string) (*git.Repository, error) {

	if verbose {
		fmt.Printf("cloning %s into %s", url, dir)
	}

	auth, keyErr := ssh.NewSSHAgentAuth("git")

	if keyErr != nil {
		return nil, keyErr
	}

	cloneOpts := &git.CloneOptions{
		Progress: gitopts.progress,
		URL:      url,
		Auth:     auth,
	}

	repo, err := git.PlainClone(dir, false, cloneOpts)

	if err != nil {
		return nil, err
	}

	return repo, nil
}

func publicKey(keyname string) (*ssh.PublicKeys, error) {

	var publicKey *ssh.PublicKeys

	sshPath := os.Getenv("HOME") + "/.ssh/" + keyname

	sshKey, err := ioutil.ReadFile(sshPath)

	if err != nil {
		return nil, err
	}

	publicKey, err = ssh.NewPublicKeys("git", []byte(sshKey), "")

	if err != nil {
		return nil, err
	}

	return publicKey, nil

}

func run() error {
	var dryrun bool = true

	// parse the user-provided git url
	gURL, err := parseGitURL(repositoryURL)
	if err != nil {
		return err
	}

	if !dryrun {
		// Create a tempDir to clone into
		tempDir, err := createTempDir()

		if err != nil {
			return fmt.Errorf("cannot create temporary directory: %s", err)
		}

		// Cleanup tempDir
		defer os.Remove(tempDir)

		// Clone the remote
		repo, err := cloneRepo(gURL.raw, tempDir)

		if err != nil {
			return fmt.Errorf("cannot clone repository: %s", err)
		}

		// Create the tag
		err = createTag(repo)
		if err != nil {
			return fmt.Errorf("cannot create tag: %s", err)
		}

		// Run a build
		err = makeBuild(tempDir)
		if err != nil {
			return fmt.Errorf("failed building artifacts: %s", err)
		}

	}

	// Create a release
	// request user & device codes
	var scope string = ""
	authResponse, err := requestDeviceAndUserCodes(githubEndpoint.DeviceAuthURL, clientID, scope)
	if err != nil {
		return fmt.Errorf("failed requesting device and user codes from github: %s", err)
	}

	// prompt user to authorize
	fmt.Printf("Please enter your one-time verification code at %s\n", authResponse.VerificationURI)
	fmt.Printf("One-time code: %s\n", authResponse.UserCode)
	openbrowser(authResponse.VerificationURI)

	// poll for auth status
	userAuthResponse, err := pollForAccessToken(
		githubEndpoint.TokenURL,
		clientID,
		authResponse.DeviceCode,
		githubDeviceGrantType,
		authResponse.ExpiresIn,
		authResponse.Interval,
	)
	if err != nil {
		return fmt.Errorf("failed checking for authorization and retrieving access token: %s", err)
	}

	// fmt.Println(userAuthResponse.AccessToken)
	fmt.Println(userAuthResponse.TokenType)
	// fmt.Println(userAuthResponse.Scope)

	releases, err := getReleases(gURL)
	fmt.Printf("RELEASES: %v\n", releases)

	resp, err := createRelease(userAuthResponse, gURL, tag, tagMessage, "", false, false)
	fmt.Printf("CREATE RELEASE RESPONSE: %+v\n", resp)

	// List releases (does one exist?)
	// https://docs.github.com/en/free-pro-team@latest/rest/reference/repos#list-releases
	// Create a Release
	// https://docs.github.com/en/free-pro-team@latest/rest/reference/repos#create-a-release
	// Upload Release Assets
	// https://docs.github.com/en/free-pro-team@latest/rest/reference/repos#upload-a-release-asset
	return nil
}

type gitURL struct {
	parsedURL    *url.URL
	organization string
	repository   string
	raw          string
}

// parseGitURL takes a gitURL string and parses it out into it's bits
func parseGitURL(repositoryURL string) (*gitURL, error) {
	var err error

	// Raw working Regex for git URLs:  (git@|(https?:\/\/))((.*)(?::|\/)(\w*)\/([\w\-]*)(?:.git)?)
	// tested against:
	// git@github.com:foo/barbazbingo.git
	// http://testing_foo.github.com/foo/barbaz_bingo_bango.git
	// https://testing3.github.com/foo/barbaz-bingo-bango.git

	expression := `(?P<scheme>git@|(https?:\/\/))(?P<host>.*)(?P<pathSeparator>:|\/)(?P<organization>\w*)\/(?P<repository>[\w\-]*)(?P<suffix>.git)?`
	re := regexp.MustCompile(expression)
	matches := re.FindStringSubmatch(repositoryURL)

	u := &gitURL{
		parsedURL: &url.URL{
			Scheme: matches[re.SubexpIndex("scheme")],
			Host:   matches[re.SubexpIndex("host")],
			Path:   formatURLPath(matches, re),
		},
		repository:   matches[re.SubexpIndex("repository")],
		organization: matches[re.SubexpIndex("organization")],
		raw:          repositoryURL,
	}

	return u, err
}

func formatURLPath(matches []string, re *regexp.Regexp) string {
	return fmt.Sprintf(matches[re.SubexpIndex("pathSeparator")] + matches[re.SubexpIndex("organization")] + "/" + matches[re.SubexpIndex("repository")] + matches[re.SubexpIndex("suffix")])
}
