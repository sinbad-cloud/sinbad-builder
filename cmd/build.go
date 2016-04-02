package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/codeskyblue/go-sh"
	"github.com/fsouza/go-dockerclient"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/util/intstr"
)

// Builder encapsulates all of the parameters necessary for starting up
// a builder. These can either be set via command line or directly.
type Builder struct {
	Author        string
	DockerMachine bool
	Dir           string
	LogJSON       bool
	Namespace     string
	Origin        string
	Registry      string
	Repo          string
	Timestamp     time.Time
	Type          string
	Verbose       bool
	Version       bool
	Zone          string
}

// NewBuilder encapsulates all of the parameters necessary for starting up
// the build job. These can either be set via command line or directly.
func NewBuilder() *Builder {
	return &Builder{}
}

// AddFlags adds flags for a specific ReBuild to the specified FlagSet
func (b *Builder) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&b.Dir, "dir", b.Dir, "Path of git repository")
	fs.BoolVar(&b.DockerMachine, "docker-machine", b.DockerMachine, "Flag to use docker-machine client")
	fs.BoolVar(&b.LogJSON, "log-json", true, "Log as JSON")
	fs.StringVar(&b.Namespace, "namespace", b.Namespace, "Git namespace (organisation or user)")
	fs.StringVar(&b.Origin, "origin", b.Origin, "Git origin e.g. github.com")
	fs.StringVar(&b.Repo, "repo", b.Repo, "Git repository")
	fs.StringVar(&b.Registry, "registry", b.Registry, "Docker registry e.g. [domain/][namespace]")
	fs.BoolVar(&b.Verbose, "verbose", false, "Verbose")
	fs.BoolVar(&b.Version, "version", false, "Print the version and exits")
	fs.StringVar(&b.Zone, "dns-zone", b.Zone, "DNS zone to which to deploy services")
}

// Run runs the job
func (b *Builder) Run() error {
	if b.Verbose {
		log.SetLevel(log.DebugLevel)
	}
	if b.LogJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	// kubernetes clones the repo at the root of the volume so we need to cd to the repo directory
	dir := path.Join(b.Dir, b.Repo)
	session := sh.NewSession()
	session.SetDir(dir)

	tag, err := shortHash(session)
	if err != nil {
		return err
	}
	image := fmt.Sprintf("%s/%s:%s", b.Registry, b.Repo, tag)

	var client *docker.Client
	if b.DockerMachine {
		client, err = docker.NewClientFromEnv()
	} else {
		client, err = docker.NewClient("unix:///var/run/docker.sock")
	}
	if err != nil {
		return err
	}

	if err = build(dir, image, client); err != nil {
		return err
	}

	authConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return err
	}
	if err = push(image, client, authConfigs.Configs["docker.atlassian.io"]); err != nil {
		return err
	}
	// TODO: create a job and use different service to deploy
	deployer, err := NewDeployer("", "", false)
	if err != nil {
		return err
	}
	envVars := make(map[string]string)
	envVars["PORT"] = "8080"
	response, err := deployer.Run(&DeployRequest{
		ContainerPort: intstr.FromInt(8080), // FIXME: hardcoding
		Environment:   "default",            // FIXME: hardcoding
		EnvVars:       envVars,
		Image:         image,
		Replicas:      1,
		ServiceID:     b.Repo,
		Zone:          b.Zone,
	})
	log.WithField("deploymentResponse", response)
	if err != nil {
		return err
	}
	return nil
}

func shortHash(s *sh.Session) (string, error) {
	output, err := s.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", err
	}
	tag := strings.Replace(string(output), "\n", "", -1)
	log.WithField("commit", tag).Debug("Short commit hash")
	return tag, nil
}

func build(src, name string, client *docker.Client) error {
	dockerfile := path.Join(src, "Dockerfile")
	var exist bool

	w := log.StandardLogger().Writer()
	defer w.Close()

	options := docker.BuildImageOptions{
		Name:         name,
		ContextDir:   src,
		OutputStream: w,
	}

	if _, err := os.Stat(dockerfile); err == nil {
		exist = true
		log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("Found existing Dockerfile"))
	} else {
		log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("Dockerfile not found, generating"))
		content := `FROM jtblin/herokuish:0.3.5
ADD . /app
RUN /bin/herokuish buildpack build
CMD ["/bin/bash", "-c", "'/start web'"]`
		if err := ioutil.WriteFile(dockerfile, []byte(content), 0644); err != nil {
			return err
		}
	}
	if !exist {
		defer func() {
			if err := os.Remove(dockerfile); err != nil {
				log.Errorf("Error removing Dockerfile: %+v", err)
			}
		}()
	}
	log.WithFields(log.Fields{"image": name}).Info(fmt.Sprintf("About to run docker build"))
	if err := client.BuildImage(options); err != nil {
		return err
	}

	return nil
}

func push(name string, client *docker.Client, auth docker.AuthConfiguration) error {
	w := log.StandardLogger().Writer()
	defer w.Close()

	log.WithFields(log.Fields{"image": name}).Info("About to push to docker registry")
	repository, tag := docker.ParseRepositoryTag(name)
	options := docker.PushImageOptions{Name: repository, Tag: tag, OutputStream: w}
	if err := client.PushImage(options, auth); err != nil {
		return err
	}
	log.WithFields(log.Fields{"image": name}).Info("Push to docker registry completed")
	return nil
}
