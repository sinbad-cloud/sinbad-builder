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
	Registry  string
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
	fs.StringVar(&r.Registry, "registry", r.Registry, "Docker registry e.g. [domain/][namespace]")
	fs.BoolVar(&r.Verbose, "verbose", false, "Verbose")
}

func (r *ReBuild) run() {
	source := fmt.Sprintf("git@%s:%s/%s.git", r.Origin, r.Namespace, r.Repo)
	dir, err := getDirectory(r)
	if err != nil {
		panic(err)
	}

	session := sh.NewSession()
	if _, err = os.Stat(path.Join(dir, ".git")); err == nil {
		if err = fetch(session, dir, source); err != nil {
			panic(err)
		}
	} else {
		if err = clone(session, dir, source); err != nil {
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

	image := fmt.Sprintf("%s/%s:%s", r.Registry, r.Repo, tag)
	output, err = build(session, dir, image, r.BuildStep)
	if err != nil {
		panic(err)
	}

	log.Info(string(output))
	log.WithFields(log.Fields{"image": image}).Info("About to push to docker registry")
	output, err = session.Command("docker", "push", image).Output()
	if err != nil {
		panic(err)
	}
	log.Info(string(output))
}

func getDirectory(r *ReBuild) (string, error) {
	if r.Dir == "" {
		return ioutil.TempDir(os.TempDir(), "rebuild")
	}
	return path.Join(r.Dir, r.Origin, r.Namespace, r.Repo), nil
}

func fetch(s *sh.Session, dir, source string) error {
	log.WithFields(log.Fields{"source": source, "dir": dir}).Info("About to fetch from upstream")
	if err := s.Call("git", "-C", dir, "fetch", "origin"); err != nil {
		return err
	}
	if err := s.Call("git", "-C", dir, "reset", "--hard"); err != nil {
		return err
	}
	return nil
}

func clone(s *sh.Session, dir, source string) error {
	log.WithFields(log.Fields{"source": source, "dir": dir}).Info("About to clone repository")
	// TODO: handle depth https://github.com/travis-ci/travis-build/blob/master/lib/travis/build/git/clone.rb#L40
	if err := s.Call("git", "clone", source, dir); err != nil {
		return err
	}
	return nil
}

func build(s *sh.Session, dir, image, buildStep string) (output []byte, err error) {
	if _, err = os.Stat(path.Join(dir, "Dockerfile")); err == nil {
		log.WithFields(log.Fields{"image": image}).Info(fmt.Sprintf("Found Dockerfile, about to run docker build"))
		output, err = s.Command("docker", "build", "-t", image, ".").Output()
	} else {
		log.WithFields(log.Fields{"image": image}).Info(fmt.Sprintf("About to run %s in %s", buildStep, dir))
		output, err = s.Command(buildStep, image).Output()
	}
	return output, err
}
