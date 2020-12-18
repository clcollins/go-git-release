# go-git-release

A CLI tool to create a tag, build binary artifacts and create a Github release with a single command

Project Status: **Extreme Alpha**

Current Limitations:

1. Currently, this does not create a release or upload the artifacts, it just tags, pushes the tag and builds the artifacts
2. Currently, the "continue with an existing tag" option doesn't actually check out the tag

## Usage

```shell
# Clone the default branch of the providied repositoryURL, create an annotated tag with the provided message, and run the Make
"buildRelease" target to generate assets

./go-git-release --tag v0.1.0 --repositoryURL git@github.com:clcollins/go-git-release.go -m "This is version 0.1.0 of
go-git-release"
```

If the tag already exists, `go-git-release` will prompt whether or not to use the existing tag.

If a tag annotation message is not provided, `go-git-release` will open an editor, Git-style, and prompt the user for a message.



## Configuration

Command line flags can alternatively be privided via a configuration file or environment variables.

Order precidence is as follows:

1. command line flags
2. environment variables
3. config file values

### Environment variables

Uppercased environment variables matching the flag names show in the help output will be parsed upon execution. For instance, in the example below, the environment variable `REPOSITORYURL` will be used for the `repositoryURL` CLI flag.

```txt
REPOSITORYURL=git@github.com/foo/bar.git go-git-release
```

### Confg File

Configuration flags will be read from a YAML config file specified by the `--config` or `-c` flags, or by default a `.go-git-release.yaml` file in the current working directory, if it exists.

Values should match the long-form flag name shown in the help output.

## Acknowledgements

A large part of this code base is cribbed from:

* Liviu Costea:
    [https://medium.com/@clm160/tag-example-with-go-git-library-4377a84bbf17](https://medium.com/@clm160/tag-example-with-go-git-library-4377a84bbf17)
* Sam Rapaport:
    [https://samrapdev.com/capturing-sensitive-input-with-editor-in-golang-from-the-cli/](https://samrapdev.com/capturing-sensitive-input-with-editor-in-golang-from-the-cli/)
