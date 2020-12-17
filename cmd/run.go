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
	"os"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
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

func confirm(s string) bool {
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
	// Create a tempDir to clone into
	tempDir, err := createTempDir()

	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %s", err)
	}

	// Cleanup tempDir
	defer os.Remove(tempDir)

	// Clone the remote
	repo, err := cloneRepo(repositoryURL, tempDir)

	if err != nil {
		return fmt.Errorf("cannot clone repository: %s", err)
	}

	// Get the repoConfig to find the username and email
	repoConfig, err := repo.ConfigScoped(config.GlobalScope)

	// Prompt for an annotationMessage
	// TODO: Not if one is provided?
	var input string
	if tagMessage != "" {
		input = tagMessage
	} else {
		i, err := captureInputFromEditor(getPreferredEditorFromEnvironment)
		if err != nil {
			return err
		}
		input = string(i)
	}

	annotationMessage := stripComments(string(input))

	tagged, err := setTag(
		repo,
		tag,
		annotationMessage,
		defaultSignature(
			repoConfig.User.Name,
			repoConfig.User.Email,
		),
	)

	if err != nil {
		return fmt.Errorf("failed creating tag: %s", err)
	}

	if tagged {
		err = pushTags(repo)

		if err != nil {
			return fmt.Errorf("failed pushing tag to remote: %s", err)
		}
	}

	return nil

}
