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
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

// TODO: make these flags
var name string = "Chris Collins"
var email string = "collins.christopher@gmail.com"

var cfgFile string
var verbose bool
var privateKey string
var repositoryURL string
var tag string
var home string

// See below - does this need to be a cli flag?
var remote string = "origin"

// gitopts holds config info for git operations
// and is parsed during init for package cmd
var gitopts struct {
	progress *os.File
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-git-release",
	Short: "Create a tag, build artifacts and a release for a Github project",
	Long: `go-git-release is a tool for tagging, building artifacts, and creating a Github release for a project with
a single command. At the moment, a Makefile with a "build" target is required.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		// Parse viper flags
		cfg := viper.AllSettings()

		// Show the input if verbose
		if verbose {
			fmt.Println("Using settings:")
			for k, v := range cfg {
				fmt.Printf("\t%v: %v\n", k, v)
			}
			fmt.Printf("\n")
		}

		verbose = viper.GetBool("verbose")
		privateKey = viper.GetString("privateKey")
		tag = viper.GetString("tag")
		repositoryURL = viper.GetString("repositoryURL")

		// Set git to write to stdout for verbose output
		if verbose {
			gitopts.progress = os.Stdout
		}
	},

	// Uncomment the following line if your bare application
	// has an action associated with it:
	RunE: func(cmd *cobra.Command, args []string) error {
		err := run()
		if err != nil {
			return err
		}
		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Get the user homedir
func init() {
	var err error
	home, err = homedir.Dir()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Optional config file for options with defaults (privateKey, remote, etc)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./.go-git-release.yaml", "path to (optional) config file")

	// Enable verbose output
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Path to private key; defaults to "id_rsa"
	rootCmd.PersistentFlags().StringVarP(&privateKey, "privateKey", "k", "id_rsa", "SSH private key to use for authentication")

	// Name of git remote to tag/push/release; "defaults to upstream"
	// Do we need this? If we're cloning the repo to a temp dir, it'll always be "origin".
	// Or do we want to act on a clone in the cwd?
	// rootCmd.PersistentFlags().StringVarP(&remote, "remote", "R", "upstream", "git remote to act on")

	// Tag name; required
	rootCmd.PersistentFlags().StringVarP(&tag, "tag", "t", "", "tag to create or use for the release")

	// Repository; required
	rootCmd.PersistentFlags().StringVarP(&repositoryURL, "repositoryURL", "r", "", "repository url")

	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("privateKey", rootCmd.PersistentFlags().Lookup("privateKey"))
	viper.BindPFlag("tag", rootCmd.PersistentFlags().Lookup("tag"))
	viper.BindPFlag("repositoryURL", rootCmd.PersistentFlags().Lookup("repositoryURL"))

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find current directory.
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in current directory with name ".go-git-release.yaml"
		viper.AddConfigPath(dir)
		viper.SetConfigName(".go-git-release.yaml")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
