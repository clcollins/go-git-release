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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
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

func setTag(repo *git.Repository, tag string, tagger *object.Signature) (bool, error) {

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

	// TODO: Implement this
	//input, err := captureInputFromEditor()

	message := fmt.Sprintf("Creating tag %s", tag) // This is the message to update with the prompt

	createOpts := &git.CreateTagOptions{
		Tagger:  tagger,
		Message: message,
	}

	if verbose {
		fmt.Printf(
			"\ntag %s\n"+
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

func run() error {
	input, err := captureInputFromEditor(getPreferredEditorFromEnvironment)
	if err != nil {
		return err
	}

	fmt.Println(input)

	os.Exit(1)

	tempDir, err := createTempDir()

	if err != nil {
		return fmt.Errorf("cannot create temporary directory: %s", err)
	}

	// Cleanup tempDir
	defer os.Remove(tempDir)

	repo, err := cloneRepo(repositoryURL, tempDir)

	if err != nil {
		return fmt.Errorf("cannot clone repository: %s", err)
	}

	repoConfig, err := repo.ConfigScoped(config.GlobalScope)

	tagged, err := setTag(
		repo,
		tag,
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

func captureInputFromEditor(resolveEditor preferredEditorResolver) ([]byte, error) {
	tempFile, err := createTempFile()
	defer os.Remove(tempFile.Name())

	if err != nil {
		return []byte{}, err
	}

	fileName := tempFile.Name()

	msg := "\n\n\n# Please enter the tag message for your annotated tag. Lines starting\n" +
		"# with '#' will be ignored, and an empty message aborts the tagging."

	err = ioutil.WriteFile(fileName, []byte(msg), 0644)
	if err != nil {
		return []byte{}, err
	}

	if err = tempFile.Close(); err != nil {
		return []byte{}, err
	}

	if err = openFileInEditor(fileName, resolveEditor); err != nil {
		return []byte{}, err
	}

	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return []byte{}, err
	}

	return bytes, nil
}

func openFileInEditor(filename string, resolveEditor preferredEditorResolver) error {
	// Get the full executable path for the editor.
	executable, err := exec.LookPath(resolveEditor())
	if err != nil {
		return err
	}

	cmd := exec.Command(executable, filename)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// PreferredEditorResolver is a function that returns an editor that the user
// prefers to use, such as the configured `$EDITOR` environment variable.
type preferredEditorResolver func() string

// getPreferredEditorFromEnvironment returns the user's editor as defined by the
// `$EDITOR` environment variable, or the `DefaultEditor` if it is not set.
func getPreferredEditorFromEnvironment() string {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = defaultEditor
	}

	return editor
}

func generateTagMessageFromTemplate() (*template.Template, error) {
	template, err := template.New("tagMessage").Parse(
		"tag {{ tag }} " +
			"Tagger: {{ name }} <{{ email }}>" +
			"Date:   {{ date }}" +
			"" +
			"{{ tagMessage }}",
	)

	if err != nil {
		return template, err
	}

	return template, nil
}
