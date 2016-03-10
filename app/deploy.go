package app

import (
	log "github.com/Sirupsen/logrus"

	"fmt"

	"k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	deploymentKey     string = "micros.atlassian.io/podTemplateHash"
	k8sAPIVersion     string = "v1"
	k8sBetaAPIVersion string = "extensions/v1beta1"
)

// Deployer is a representation of a job runner
type Deployer struct {
	Client *client.Client
}

// DeployRequest represents the request payload
type DeployRequest struct {
	Args          []string         `json:"arguments"`
	ContainerPort util.IntOrString `json:"containerPort"`
	Environment   string           `json:"environment"`
	EnvVars   map[string]string           `json:"envVars"`
	Heartbeat     struct {
			      Path                         string           `json:"path"`
			      Port                         util.IntOrString `json:"port"`
			      InitialDelayLivenessSeconds  int64            `json:"initialDelayLivenessSeconds"`
			      InitialDelayReadinessSeconds int64            `json:"initialDelayReadinessSeconds"`
			      TimeoutSeconds               int64            `json:"timeoutSeconds"`
		      } `json:"heartbeat"`
	Image     string `json:"image"`
	Replicas  int    `json:"replicas"`
	ServiceID string `json:"serviceId"`
	Secrets   []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"secrets"`
	Tags map[string]string `json:"tags"`
	Zone string            `json:"zone"`
}

// DeployResponse represents the response payload
type DeployResponse struct {
	Request  DeployRequest `json:"request"`
	NodePort int           `json:"nodePort"`
}

// NewDeployer creates a new deployer
func NewDeployer(host, token string, insecure bool) (*Deployer, error) {
	var c *client.Client
	var err error
	if host != "" && token != "" {
		config := &client.Config{
			Host:        host,
			BearerToken: token,
			Insecure:    insecure,
		}
		c, err = client.New(config)
	} else {
		c, err = client.NewInCluster()
	}
	if err != nil {
		return nil, err
	}
	return &Deployer{c}, nil
}

// Run runs the deployment
func (d *Deployer) Run(payload *DeployRequest) (*DeployResponse, error) {
	res := &DeployResponse{Request: *payload}

	// create namespace if needed
	if _, err := d.Client.Namespaces().Create(newNamespace(payload)); err != nil {
		if !apierrs.IsAlreadyExists(err) {
			return nil, err
		}
	}

	// create service
	svc, err := d.CreateOrUpdateService(newService(payload), payload.Environment)
	if err != nil {
		return res, err
	}

	if len(svc.Spec.Ports) > 0 {
		res.NodePort = svc.Spec.Ports[0].NodePort
	}

	// create deployment
	deployment, err := d.CreateOrUpdateDeployment(newDeployment(payload), payload.Environment)
	if err != nil {
		return res, err
	}

	// get deployment status
	selector := labels.Everything().Add("name", labels.ExistsOperator, []string{payload.ServiceID})
	watcher, err := d.Client.Deployments(payload.Environment).Watch(selector, fields.Everything(), deployment.ResourceVersion)
	if err != nil {
		return res, err
	}
	// TODO: timeout?
	d.WatchLoop(watcher, func(e watch.Event) bool {
		switch e.Type {
		case watch.Modified:
			d, err := d.Client.Deployments(payload.Environment).Get(payload.ServiceID)
			if err != nil {
				log.Errorf("Error getting deployment: %+v", err)
				return true
			}
			log.Debugf("Modified deployment: %+v", d)
			if d.Status.Replicas == d.Spec.Replicas {
				return true
			}
		}
		return false
	})

	_, err = d.CreateOrUpdateIngress(newIngress(res), payload.Environment)
	if err != nil {
		return res, err
	}

	log.Infof("Deployment completed: %+v", svc)
	return res, nil
}

// WatchLoop loops, passing events in w to fn.
func (r *Deployer) WatchLoop(w watch.Interface, fn func(watch.Event) bool) {
	for {
		select {
		case event, ok := <-w.ResultChan():
			if !ok {
				log.Info("No more events")
				return
			}
			log.Debugf("Received event: %+v", event)
			if stop := fn(event); stop {
				w.Stop()
			}
		}
	}
}

func newNamespace(payload *DeployRequest) *api.Namespace {
	return &api.Namespace{
		ObjectMeta: api.ObjectMeta{Name: payload.Environment},
		TypeMeta:   unversioned.TypeMeta{APIVersion: k8sAPIVersion, Kind: "Namespace"},
	}
}

// CreateOrUpdateService creates or updates a service
func (r *Deployer) CreateOrUpdateService(svc *api.Service, env string) (*api.Service, error) {
	newsSvc, err := r.Client.Services(env).Create(svc)
	if err != nil {
		if !apierrs.IsAlreadyExists(err) {
			return nil, err
		}
		oldSvc, err := r.Client.Services(env).Get(svc.ObjectMeta.Name)
		if err != nil {
			return nil, err
		}
		svc.ObjectMeta.ResourceVersion = oldSvc.ObjectMeta.ResourceVersion
		svc.Spec.ClusterIP = oldSvc.Spec.ClusterIP
		svc.Spec.Ports[0].NodePort = oldSvc.Spec.Ports[0].NodePort
		svc, err = r.Client.Services(env).Update(svc)
		if err != nil {
			return nil, err
		}
		log.Debugf("Service updated: %+v", svc)
		return svc, nil
	}
	log.Debugf("Service created: %+v", svc)
	return newsSvc, nil
}

func newService(payload *DeployRequest) *api.Service {
	return &api.Service{
		ObjectMeta: newMetadata(payload),
		Spec: api.ServiceSpec{
			Type: api.ServiceTypeNodePort,
			Ports: []api.ServicePort{{
				Port: payload.ContainerPort.IntVal,
			}},
			Selector: map[string]string{"name": payload.ServiceID},
		},
		TypeMeta: unversioned.TypeMeta{APIVersion: k8sAPIVersion, Kind: "Service"},
	}
}

func newMetadata(payload *DeployRequest) api.ObjectMeta {
	return api.ObjectMeta{
		Annotations: payload.Tags,
		Labels:      map[string]string{"name": payload.ServiceID},
		Name:        payload.ServiceID,
		Namespace:   payload.Environment,
	}
}

// CreateOrUpdateDeployment creates or updates a service
func (r *Deployer) CreateOrUpdateDeployment(d *extensions.Deployment, env string) (*extensions.Deployment, error) {
	log.WithFields(log.Fields{"d": d, "image": d.Spec.Template.Spec.Containers[0].Image}).Debug("New deployment")

	newD, err := r.Client.Deployments(env).Create(d)
	if err != nil {
		log.Debugf("Error: %s", err.Error())
		if !apierrs.IsAlreadyExists(err) {
			return nil, err
		}
		d, err = r.Client.Deployments(env).Update(d)
		if err != nil {
			return nil, err
		}
		log.Debugf("Deployment updated: %+v", d)
		return d, nil

	}
	log.Debugf("Deployment created: %+v", d)
	return newD, nil
}

func newDeployment(payload *DeployRequest) *extensions.Deployment {
	return &extensions.Deployment{
		ObjectMeta: newMetadata(payload),
		Spec: extensions.DeploymentSpec{
			Replicas: payload.Replicas,
			Selector: map[string]string{"name": payload.ServiceID},
			Strategy: extensions.DeploymentStrategy{
				Type: extensions.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &extensions.RollingUpdateDeployment{
					MaxUnavailable:  util.NewIntOrStringFromString("10%"),
					MaxSurge:        util.NewIntOrStringFromString("10%"),
					MinReadySeconds: 15,
				},
			},
			Template: &api.PodTemplateSpec{
				ObjectMeta: newMetadata(payload),
				Spec: api.PodSpec{
					// TODO: disable ServiceAccountName
					Containers: []api.Container{
						api.Container{
							Args:  payload.Args,
							Name:  payload.ServiceID,
							Image: payload.Image,
							Ports: []api.ContainerPort{{
								Name:          "http",
								ContainerPort: payload.ContainerPort.IntVal,
							}},
							//LivenessProbe: newProbe(payload, payload.Heartbeat.InitialDelayLivenessSeconds),
							//// FIXME: payload.Heartbeat.InitialDelayReadinessSeconds does not work as expected
							//// makes instances fail health check if set to micros 1200s default value
							//// as no traffic is routed until the readiness check passes
							//// but that doesn't prevent the pods to be replaced...
							//ReadinessProbe: newProbe(payload, 15),
							//Resources: api.ResourceRequirements{
							//	Limits: api.ResourceList{
							//		api.ResourceCPU:    payload.Resources.ParsedCPU,
							//		api.ResourceMemory: payload.Resources.ParsedMemory,
							//	},
							//},
						},
					},
					RestartPolicy: "Always",
				},
			},
			UniqueLabelKey: deploymentKey,
		},
		TypeMeta: unversioned.TypeMeta{APIVersion: k8sBetaAPIVersion, Kind: "Deployment"},
	}
}

func newProbe(payload *DeployRequest, delay int64) *api.Probe {
	return &api.Probe{
		Handler: api.Handler{HTTPGet: &api.HTTPGetAction{
			Path: payload.Heartbeat.Path,
			Port: payload.ContainerPort,
		}},
		InitialDelaySeconds: delay,
		TimeoutSeconds:      payload.Heartbeat.TimeoutSeconds,
	}
}

// CreateOrUpdateIngress creates or updates an ingress rule
func (r *Deployer) CreateOrUpdateIngress(ingress *extensions.Ingress, env string) (*extensions.Ingress, error) {
	newIngress, err := r.Client.Ingress(env).Create(ingress)
	if err != nil {
		log.Debugf("Error: %s", err.Error())
		if !apierrs.IsAlreadyExists(err) {
			return nil, err
		}
		ingress, err = r.Client.Ingress(env).Update(ingress)
		if err != nil {
			return nil, err
		}
		log.Debugf("Ingress updated: %+v", ingress)
		return ingress, nil

	}
	log.Debugf("Ingress created: %+v", ingress)
	return newIngress, nil
}

func newIngress(payload *DeployResponse) *extensions.Ingress {
	r := payload.Request
	return &extensions.Ingress{
		ObjectMeta: newMetadata(&payload.Request),
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{{
				Host: fmt.Sprintf("%s.%s", r.ServiceID, r.Zone),
				IngressRuleValue: extensions.IngressRuleValue{HTTP: &extensions.HTTPIngressRuleValue{
					Paths: []extensions.HTTPIngressPath{{Path: "/", Backend: extensions.IngressBackend{
						ServiceName: r.ServiceID,
						ServicePort: r.ContainerPort,
					}}},
				}},
			}},
		},
		TypeMeta: unversioned.TypeMeta{APIVersion: k8sBetaAPIVersion, Kind: "Ingress"},
	}
}
