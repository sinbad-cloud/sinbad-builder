package app

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
	kutil "k8s.io/kubernetes/pkg/util"
)

// ReBuild encapsulates all of the parameters necessary for starting up
// a builder. These can either be set via command line or directly.
type ReBuild struct {
	Author        string
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

// AddFlags adds flags for a specific ReBuild to the specified FlagSet
func (r *ReBuild) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&r.Dir, "dir", r.Dir, "Path of git repository")
	fs.BoolVar(&r.DockerMachine, "docker-machine", r.DockerMachine, "Flag to use docker-machine client")
	fs.StringVar(&r.Namespace, "namespace", r.Namespace, "Namespace")
	fs.StringVar(&r.Origin, "origin", r.Origin, "Origin e.g. github.com")
	fs.StringVar(&r.Repo, "repo", r.Repo, "Git repository")
	fs.StringVar(&r.Registry, "registry", r.Registry, "Docker registry e.g. [domain/][namespace]")
	fs.BoolVar(&r.Verbose, "verbose", false, "Verbose")
}

// Run runs the job
func (r *ReBuild) Run() error {
	// kubernetes clone the repo at the root of the volume so need to cd to the repo directory
	dir := path.Join(r.Dir, r.Repo)
	session := sh.NewSession()
	session.SetDir(dir)

	tag, err := shortHash(session)
	if err != nil {
		return err
	}
	image := fmt.Sprintf("%s/%s:%s", r.Registry, r.Repo, tag)

	var client *docker.Client
	if r.DockerMachine {
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
	if err = push(image, client, authConfigs.Configs["https://index.docker.io/v1/"]); err != nil {
		return err
	}
	// TODO: use different service
	deployer, err := NewDeployer("", "", false)
	if err != nil {
		return err
	}
	envVars := make(map[string]string)
	envVars["PORT"] = "8080"
	response, err := deployer.Run(&DeployRequest{
		ContainerPort: kutil.NewIntOrStringFromInt(8080), // FIXME: hardcoding
		Environment: "default", // FIXME: hardcoding
		EnvVars: envVars,
		Image: image,
		Replicas: 1,
		ServiceID: r.Repo,
		Zone: "atlassianapp.cloud", // FIXME: hardcoding
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
	log.WithFields(log.Fields{"commit": string(tag)}).Debug("Short commit hash")
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
