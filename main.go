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
	"github.com/codeskyblue/go-sh"
	"github.com/spf13/pflag"
)

// ReBuild encapsulates all of the parameters necessary for starting up
// a builder. These can either be set via command line or directly.
type ReBuild struct {
	Author    string
	BuildStep string
	Commit    string
	Namespace string
	Origin    string
	Dir       string
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
	fs.StringVar(&r.Commit, "commit", r.Commit, "Commit to checkout")
	fs.StringVar(&r.Dir, "dir", r.Dir, "Directory to clone repositories")
	fs.StringVar(&r.Namespace, "namespace", r.Namespace, "Namespace")
	fs.StringVar(&r.Origin, "origin", r.Origin, "Origin e.g. github.com")
	fs.StringVar(&r.Repo, "repo", r.Repo, "Git repository")
	fs.BoolVar(&r.Verbose, "verbose", false, "Verbose")
}

func (r *ReBuild) run() {
	repo := fmt.Sprintf("git@%s:%s/%s.git", r.Origin, r.Namespace, r.Repo)
	dir, err := getDirectory(r.Dir, r.Namespace, r.Repo)
	if err != nil {
		panic(err)
	}

	session := sh.NewSession()
	if _, err = os.Stat(path.Join(dir, ".git")); err == nil {
		if err = fetch(session, dir, repo); err != nil {
			panic(err)
		}
	} else {
		if err = clone(session, dir, repo); err != nil {
			panic(err)
		}
	}
	session.SetDir(dir)
	if err = session.Call("git", "checkout", "-qf", r.Commit); err != nil {
		panic(err)
	}

	output, err := session.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		panic(err)
	}
	tag := strings.Replace(string(output), "\n", "", -1)
	log.WithFields(log.Fields{"commit": string(tag)}).Debug("Short commit hash")

	log.Info(fmt.Sprintf("About to run %s in %s", r.BuildStep, dir))
	output, err = session.Command(r.BuildStep, r.Repo, tag).Output()
	if err != nil {
		panic(err)
	}
	log.Debug("buildstep " + string(output))
}

func getDirectory(dir, namespace, repo string) (string, error) {
	if dir == "" {
		return ioutil.TempDir(os.TempDir(), "rebuild")
	}
	return path.Join(dir, namespace, repo), nil
}

func fetch(s *sh.Session, dir, repo string) error {
	log.WithFields(log.Fields{"repository": repo, "dir": dir}).Info("About to fetch from upstream")
	if err := s.Call("git", "-C", dir, "fetch", "origin"); err != nil {
		return err
	}
	if err := s.Call("git", "-C", dir, "reset", "--hard"); err != nil {
		return err
	}
	return nil
}

func clone(s *sh.Session, dir, repo string) error {
	log.WithFields(log.Fields{"repository": repo, "dir": dir}).Info("About to clone repository")
	// TODO: handle depth https://github.com/travis-ci/travis-build/blob/master/lib/travis/build/git/clone.rb#L40
	if err := s.Call("git", "clone", repo, dir); err != nil {
		return err
	}
	return nil
}
