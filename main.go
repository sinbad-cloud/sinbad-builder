package main

import (
	"runtime"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"

	"github.com/sinbad-cloud/sinbad-builder/cmd"
	"github.com/sinbad-cloud/sinbad-builder/version"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	b := cmd.NewBuilder()
	b.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if b.Version {
		version.PrintVersionAndExit()
	}

	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
