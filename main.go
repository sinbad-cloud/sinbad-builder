package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codeskyblue/go-sh"
	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/pflag"
)

// ReBuild encapsulates all of the parameters necessary for starting up
// a builder. These can either be set via command line or directly.
type ReBuild struct {
	Author        string
	BuildStep     string
	Commit        string
	Namespace     string
	Origin        string
	Dir           string
	DockerMachine bool
	Repo          string
	Registry      string
	Timestamp     time.Time
	Type          string
	Verbose       bool
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
	fs.BoolVar(&r.DockerMachine, "docker-machine", r.DockerMachine, "Flag to use docker-machine client")
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

	var client *docker.Client
	if r.DockerMachine {
		client, err = docker.NewClientFromEnv()
	} else {
		client, err = docker.NewClient("unix:///var/run/docker.sock")
	}
	if err != nil {
		log.Panic(err)
	}

	w := log.StandardLogger().Writer()
	defer w.Close()

	if err = build(dir, image, client, w); err != nil {
		log.Panic(err)
	}

	authConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		log.Panic(err)
	}
	log.Debugf("Docker auth configurations: %+v", authConfigs)
	if err = push(image, client, authConfigs.Configs["https://index.docker.io/v1/"], w); err != nil {
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
	}
	return clone(s, dir, source)
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

func build(src, name string, client *docker.Client, out io.Writer) error {
	dockerfile := path.Join(src, "Dockerfile")
	var exist bool

	options := docker.BuildImageOptions{
		Name:         name,
		ContextDir:   src,
		OutputStream: out,
	}

	if _, err := os.Stat(dockerfile); err == nil {
		exist = true
		log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("Found existing Dockerfile"))
	} else {
		log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("Dockerfile not found, generating"))
		content := `FROM progrium/buildstep
ADD . /app
RUN /build/builder
CMD ["/bin/bash", "-c", "'/start web'"]`
		if err := ioutil.WriteFile(dockerfile, []byte(content), 0644); err != nil {
			return err
		}
	}
	log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("About to run docker build"))
	if err := client.BuildImage(options); err != nil {
		return err
	}
	if !exist {
		if err := os.Remove(dockerfile); err != nil {
			return err
		}
	}
	return nil
}

func push(name string, client *docker.Client, auth docker.AuthConfiguration, out io.Writer) error {
	log.WithFields(log.Fields{"image": name}).Info("About to push to docker registry")
	repository, tag := docker.ParseRepositoryTag(name)
	options := docker.PushImageOptions{Name: repository, Tag: tag, OutputStream: out}
	if err := client.PushImage(options, auth); err != nil {
		return err
	}
	return nil
}
