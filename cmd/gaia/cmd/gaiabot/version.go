package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version of forwarder
	Version = "0.0.0"

	// GitCommit is the current HEAD set using ldflags.
	GitCommit string

	// GoVersion is version info of golang
	GoVersion string

	// BuidDate is compile date and time
	BuidDate string

	// Blockchain version info
	Blockchain string
)

// NewVersionCommand create version command
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			return versioner()
		},
	}
	return cmd
}

var versioner = func() error {

	s := `gaiabot - a robot for blockchain cosmos
---------------------------------------
version:	%s
revision:	%s
compile:	%s
go version:	%s
blockchain:	%s
`

	fmt.Printf(s, Version, GitCommit, BuidDate, GoVersion, Blockchain)

	return nil
}
