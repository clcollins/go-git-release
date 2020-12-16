# go-git-release

A CLI tool to create a tag, build binary artifacts and create a Github release with a single command

Project Status: **Extreme Alpha**

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
