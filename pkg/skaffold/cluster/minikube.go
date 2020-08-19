package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/constants"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubernetes"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubernetes/context"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

// IsMinikube returns true if the given kubeContext maps to a minikube cluster
func IsMinikube(kubeContext string) (bool, error) {
	// short circuit if context is 'minikube'
	if kubeContext == constants.DefaultMinikubeContext {
		return true, nil
	}
	_, err := minikubeBinary()
	if err != nil {
		return false, nil // minikube binary not found
	}

	if ok, err := matchNodeLabel(); err != nil {
		return false, nil
	} else if ok {
		logrus.Debugf("Detected minikube cluster for context %s due to matched labels", kubeContext)
		return true, nil
	}

	if ok, err := matchProfileAndServerUrl(kubeContext); err != nil {
		return false, nil
	} else if ok {
		logrus.Debugf("Detected minikube cluster for context %s due to matched profile name or server url", kubeContext)
		return true, nil
	}
	return false, nil
}

// MinikubeExec returns the Cmd struct to execute minikube with given arguments
func MinikubeExec(arg ...string) (*exec.Cmd, error) {
	b, err := minikubeBinary()
	if err != nil {
		return nil, fmt.Errorf("getting minikube executable: %w", err)
	}
	return exec.Command(b, arg...), nil
}

func minikubeBinary() (string, error) {
	execName := "minikube"
	if found, _ := util.DetectWSL(); found {
		execName = "minikube.exe"
	}
	filename, err := exec.LookPath(execName)
	if err != nil {
		return "", errors.New("unable to find minikube executable. Please add it to PATH environment variable")
	}
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return "", fmt.Errorf("unable to find minikube executable. File not found %s", filename)
	}
	return filename, nil
}

func matchNodeLabel() (bool, error) {
	client, err := kubernetes.Client()
	if err != nil {
		return false, fmt.Errorf("getting Kubernetes client: %w", err)
	}
	opts := v1.ListOptions{
		LabelSelector: "minikube.k8s.io/name=minikube",
		Limit:         1,
	}
	l, err := client.CoreV1().Nodes().List(opts)
	if err != nil {
		return false, fmt.Errorf("listing nodes with matching label: %w", err)
	}
	return l != nil && len(l.Items) > 0, nil
}

// matchProfileAndServerUrl checks if kubecontext matches any valid minikube profile
// and for selected drivers if the k8s server url is same as any of the minikube nodes IPs
func matchProfileAndServerUrl(kubeContext string) (bool, error) {
	config, err := context.GetRestClientConfig()
	if err != nil {
		return false, fmt.Errorf("getting kubernetes config: %w", err)
	}
	apiServerUrl, _, err := rest.DefaultServerURL(config.Host, config.APIPath, schema.GroupVersion{}, false)

	if err != nil {
		return false, fmt.Errorf("getting kubernetes server url: %w", err)
	}

	logrus.Debugf("kubernetes server url: %s", apiServerUrl)

	ok, err := matchServerUrlFor(kubeContext, apiServerUrl)
	if err != nil {
		return false, fmt.Errorf("checking minikube node url: %w", err)
	}
	return ok, nil
}

func matchServerUrlFor(profileName string, serverUrl *url.URL) (bool, error) {
	cmd, err := MinikubeExec("profile", "list", "-o", "json")
	if err != nil {
		return false, fmt.Errorf("executing minikube command: %w", err)
	}

	out, err := util.RunCmdOut(cmd)
	if err != nil {
		return false, fmt.Errorf("getting minikube profiles: %w", err)
	}

	var data data
	if err = json.Unmarshal(out, &data); err != nil {
		log.Fatal(fmt.Errorf("failed to unmarshal data: %w", err))
	}

	for _, v := range data.Valid {
		if v.Config.Name != profileName {
			continue
		}

		if v.Config.Driver != "hyperkit" && v.Config.Driver != "virtualbox" {
			// Since node IPs don't match server API for other drivers we assume profile name match is enough.
			// TODO: Revisit once https://github.com/kubernetes/minikube/issues/6642 is fixed
			return true, nil
		}
		for _, n := range v.Config.Nodes {
			if serverUrl.Host == fmt.Sprintf("%s:%d", n.IP, n.Port) {
				return true, nil
			}
		}
	}
	return false, nil
}

type data struct {
	Valid   []profile `json:"valid,omitempty"`
	Invalid []profile `json:"invalid,omitempty"`
}

type profile struct {
	Config config
}

type config struct {
	Name   string
	Driver string
	Nodes  []node
}

type node struct {
	IP   string
	Port int32
}
