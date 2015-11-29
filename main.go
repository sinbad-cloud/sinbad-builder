package main

import (
	"runtime"

	"bitbucket.org/jtblin/rebuild/app"
	log "github.com/Sirupsen/logrus"
	"github.com/spf13/pflag"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	r := &app.ReBuild{}
	r.AddFlags(pflag.CommandLine)
	pflag.Parse()

	if r.Verbose {
		log.SetLevel(log.DebugLevel)
	}
	if err := r.Run(); err != nil {
		log.Fatalf("%v\n", err)
	}
}
