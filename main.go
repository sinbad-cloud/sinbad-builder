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
	source := fmt.Sprintf("https://%s/%s/%s.git", r.Origin, r.Namespace, r.Repo)
	dir, err := getDirectory(r)
	if err != nil {
		log.Panic(err)
	}

	session := sh.NewSession()
	if err = fetchOrClone(session, dir, source); err != nil {
		log.Panic(err)
	}
	session.SetDir(dir)
	if err = checkout(session, r.Commit); err != nil {
		log.Panic(err)
	}

	tag, err := shortHash(session)
	if err != nil {
		log.Panic(err)
	}
	image := fmt.Sprintf("%s/%s:%s", r.Registry, r.Repo, tag)

	if _, err = build(session, dir, image, r.BuildStep); err != nil {
		log.Panic(err)
	}

	if _, err = push(session, image); err != nil {
		log.Panic(err)
	}
}

func getDirectory(r *ReBuild) (string, error) {
	if r.Dir == "" {
		return ioutil.TempDir(os.TempDir(), "rebuild")
	}
	return path.Join(r.Dir, r.Origin, r.Namespace, r.Repo), nil
}

func fetchOrClone(s *sh.Session, dir, source string) error {
	if _, err := os.Stat(path.Join(dir, ".git")); err == nil {
		return fetch(s, dir, source)
	} else {
		return clone(s, dir, source)
	}
}

func fetch(s *sh.Session, dir, source string) error {
	log.WithFields(log.Fields{"source": source, "dir": dir}).Info("About to fetch from upstream")
	if err := s.Call("git", "-C", dir, "fetch", "origin"); err != nil {
		return err
	}
	return s.Call("git", "-C", dir, "reset", "--hard")
}

func clone(s *sh.Session, dir, source string) error {
	log.WithFields(log.Fields{"source": source, "dir": dir}).Info("About to clone repository")
	// TODO: handle depth https://github.com/travis-ci/travis-build/blob/master/lib/travis/build/git/clone.rb#L40
	return s.Call("git", "clone", source, dir)
}

func checkout(s *sh.Session, commit string) error {
	return s.Call("git", "checkout", "-qf", commit)
}

func shortHash(s *sh.Session) (string, error) {
	output, err := s.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", err
	}
	tag := strings.Replace(string(output), "\n", "", -1)
	log.WithFields(log.Fields{"commit": string(tag)}).Debug("Short commit hash")
	return tag, nil
}

func build(s *sh.Session, dir, image, buildStep string) (output []byte, err error) {
	if _, err = os.Stat(path.Join(dir, "Dockerfile")); err == nil {
		log.WithFields(log.Fields{"image": image}).Info(fmt.Sprintf("Found Dockerfile, about to run docker build"))
		output, err = s.Command("docker", "build", "-t", image, ".").Output()
	} else {
		log.WithFields(log.Fields{"image": image}).Info(fmt.Sprintf("About to run %s in %s", buildStep, dir))
		output, err = s.Command(buildStep, image).Output()
	}
	log.Info(string(output))
	return
}

func push(s *sh.Session, image string) (output []byte, err error) {
	log.WithFields(log.Fields{"image": image}).Info("About to push to docker registry")
	output, err = s.Command("docker", "push", image).Output()
	if err != nil {
		return
	}
	log.Info(string(output))
	return
}
