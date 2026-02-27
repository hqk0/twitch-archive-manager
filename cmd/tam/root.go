package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tam",
	Short: "Twitch Archive Manager",
	Long:  `Twitch Archive Manager (tam) is a CLI tool to manage Twitch VOD archives.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
