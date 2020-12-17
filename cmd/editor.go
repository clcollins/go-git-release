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
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"
)

var annotatedTagPrompt string = "\n\n\n# Please enter the tag message for your annotated tag. Lines starting\n" +
	"# with '#' will be ignored, and an empty message aborts the tagging."

// generateTagMessageFromTemplate returns a string formatted to look like a completed
// Git tag message, to display to the user
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

// openFileInEditor accepts a filename and a preferredEditorResolver, and opens the file
// in the editor returned by the preferredEditorResolver.
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

// captureInputFromEditor creates a temp file and populates it with a Git commit-style
// message prompting the user to enter a message for the annotated tag and captures
// and returns the output
func captureInputFromEditor(resolveEditor preferredEditorResolver) ([]byte, error) {
	tempFile, err := createTempFile()
	defer os.Remove(tempFile.Name())

	if err != nil {
		return []byte{}, err
	}

	fileName := tempFile.Name()

	err = ioutil.WriteFile(fileName, []byte(annotatedTagPrompt), 0644)
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
