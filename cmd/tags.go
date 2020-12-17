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

// Package cmd is the root cobra command package
package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func tagExists(tag string, repo *git.Repository) (bool, error) {

	tagFoundError := "tag exists"

	tags, err := repo.TagObjects()

	if err != nil {
		return false, err
	}

	res := false

	err = tags.ForEach(func(t *object.Tag) error {
		if t.Name == tag {
			res = true
			return fmt.Errorf(tagFoundError)
		}
		return nil
	})

	if err != nil && err.Error() != tagFoundError {
		return false, err
	}

	return res, nil
}

func setTag(repo *git.Repository, tag string, message string, tagger *object.Signature) (bool, error) {

	alreadyTagged, _ := tagExists(tag, repo)
	if alreadyTagged {
		c := confirm("Tag already exists. Continue using exising tag?")
		if !c {
			// Do not continue with existing tag
			return false, errors.New("cancelled by user")
		}

		// Continue with the existing tag
		return true, nil
	}

	head, err := repo.Head()

	if err != nil {
		return false, err
	}

	createOpts := &git.CreateTagOptions{
		Tagger:  tagger,
		Message: message,
	}

	// TODO: Add the commit information so it looks like the output from Git
	// TODO: Format the date so it matches Git
	if verbose {
		fmt.Printf(
			"\ntag %s\n\n"+
				"Tagger: %s <%s>\n"+
				"Date:   %s\n"+
				"\n"+
				"%s\n"+
				"\n",
			tag,
			tagger.Name,
			tagger.Email,
			tagger.When,
			message,
		)
	}

	_, err = repo.CreateTag(tag, head.Hash(), createOpts)

	if err != nil {
		return false, err
	}

	return true, nil
}

func defaultSignature(name, email string) *object.Signature {
	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

func pushTags(repo *git.Repository) error {
	auth, err := ssh.NewSSHAgentAuth("git")

	if err != nil {
		return err
	}

	pushOpts := &git.PushOptions{
		RemoteName: remote,
		Progress:   gitopts.progress,
		RefSpecs:   []config.RefSpec{config.RefSpec("refs/tags/*:refs/tags/*")},
		Auth:       auth,
	}

	err = repo.Push(pushOpts)

	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			if verbose {
				fmt.Printf("remote %s already up to date\n", remote)
			}
			return nil
		}

		return err
	}

	return nil
}

// stripComments removes lines beginning with a "#" from the input string
func stripComments(s string) string {

	r := regexp.MustCompile("(?m)^#.*")
	return r.ReplaceAllString(s, "")

}
