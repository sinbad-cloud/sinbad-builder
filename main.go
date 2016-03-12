package main

import (
	"runtime"

	"bitbucket.org/jtblin/kigo-builder/cmd"
	"bitbucket.org/jtblin/kigo-builder/version"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
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
