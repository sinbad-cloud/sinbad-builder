package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	//	"github.com/progrium/go-basher"
	//	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/pflag"
)

// ReBuild encapsulates all of the parameters necessary for starting up
// a builder. These can either be set via command line or directly.
type ReBuild struct {
	Author    string
	BuildStep string
	Namespace string
	Origin    string
	Repo      string
	Timestamp time.Time
	Type      string
	Verbose   bool
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	r := &ReBuild{}
	r.addFlags(pflag.CommandLine)
	pflag.Parse()

	if r.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	r.run()
}

func (r *ReBuild) addFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.BuildStep, "build-step", r.BuildStep, "Location of buildstep file")
	fs.StringVar(&r.Namespace, "namespace", r.Namespace, "Namespace")
	fs.StringVar(&r.Origin, "origin", r.Origin, "Origin e.g. github.com")
	fs.StringVar(&r.Repo, "repo", r.Repo, "Git repository")
	fs.BoolVar(&r.Verbose, "verbose", false, "Verbose")
}

func (r *ReBuild) run() {
	repo := fmt.Sprintf("git@%s:%s/%s.git", r.Origin, r.Namespace, r.Repo)
	tmpDir, err := ioutil.TempDir(os.TempDir(), "rebuild")
	if err != nil {
		panic(err)
	}
	log.WithFields(log.Fields{"repository": repo, "temp": tmpDir}).Debug("About to clone repository")

	output, err := Exec(tmpDir, "git", "clone", repo)
	if err != nil {
		//		fmt.Println(err.Error())
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
		os.Exit(1)
	}

	output, err = Exec(path.Join(tmpDir, r.Repo), "git", "rev-parse", "--short", "HEAD")
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
		os.Exit(1)
	}
	tag := strings.Replace(string(output), "\n", "", -1)
	log.WithFields(log.Fields{"commit": tag}).Debug("Short commit hash")

	log.Debug("About to run", path.Join(tmpDir, r.Repo), r.BuildStep, r.Repo, tag)
	output, err = Exec(path.Join(tmpDir, r.Repo), r.BuildStep, r.Repo, tag)
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + string(output))
		os.Exit(1)
	}
	log.Debug("buildstep " + string(output))
}
