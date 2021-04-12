/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resource

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/GoogleContainerTools/skaffold/pkg/diag"
	"github.com/GoogleContainerTools/skaffold/pkg/diag/validator"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/event"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubectl"
	"github.com/GoogleContainerTools/skaffold/proto/v1"
)

const (
	deploymentType          = "deployment"
	rollOutSuccess          = "successfully rolled out"
	connectionErrMsg        = "Unable to connect to the server"
	killedErrMsg            = "signal: killed"
	defaultPodCheckDeadline = 30 * time.Second
	tabHeader               = " -"
	tab                     = "  "
	maxLogLines             = 3
)

var (
	msgKubectlKilled     = "kubectl rollout status command interrupted\n"
	MsgKubectlConnection = "kubectl connection error\n"

	nonRetryContainerErrors = map[proto.StatusCode]struct{}{
		proto.StatusCode_STATUSCHECK_IMAGE_PULL_ERR:       {},
		proto.StatusCode_STATUSCHECK_RUN_CONTAINER_ERR:    {},
		proto.StatusCode_STATUSCHECK_CONTAINER_TERMINATED: {},
		proto.StatusCode_STATUSCHECK_CONTAINER_RESTARTING: {},
	}

	RolloutTypes = struct {
		Deployment   RolloutType
		StatefulSets RolloutType
	}{
		Deployment:   "deployment",
		StatefulSets: "statefulsets",
	}
)

type RolloutType string

type Rollout struct {
	name         string
	namespace    string
	rType        RolloutType
	status       Status
	statusCode   proto.StatusCode
	done         bool
	deadline     time.Duration
	pods         map[string]validator.Resource
	podValidator diag.Diagnose
}

func (r *Rollout) Deadline() time.Duration {
	return r.deadline
}

func (r *Rollout) UpdateStatus(ae proto.ActionableErr) {
	updated := newStatus(ae)
	if r.status.Equal(updated) {
		r.status.changed = false
		return
	}
	r.status = updated
	r.statusCode = updated.ActionableError().ErrCode
	r.status.changed = true
	if ae.ErrCode == proto.StatusCode_STATUSCHECK_SUCCESS || isErrAndNotRetryAble(ae.ErrCode) {
		r.done = true
	}
}

func NewRollout(name string, rType RolloutType, ns string, deadline time.Duration) *Rollout {
	return &Rollout{
		name:         name,
		namespace:    ns,
		rType:        rType,
		status:       newStatus(proto.ActionableErr{}),
		deadline:     deadline,
		podValidator: diag.New(nil),
	}
}

func (r *Rollout) WithValidator(pd diag.Diagnose) *Rollout {
	r.podValidator = pd
	return r
}

func (r *Rollout) CheckStatus(ctx context.Context, cfg kubectl.Config) {
	kubeCtl := kubectl.NewCLI(cfg, "")

	b, err := kubeCtl.RunOut(ctx, "rollout", "status", string(r.rType), r.name, "--namespace", r.namespace, "--watch=false")
	if ctx.Err() != nil {
		return
	}

	details := r.cleanupStatus(string(b))

	ae := parseKubectlRolloutError(details, err)
	if ae.ErrCode == proto.StatusCode_STATUSCHECK_KUBECTL_PID_KILLED {
		ae.Message = fmt.Sprintf("received Ctrl-C or %s rollout could not stabilize within %v: %v", r.rType, r.deadline, err)
	}

	r.UpdateStatus(ae)
	if err := r.fetchPods(ctx); err != nil {
		logrus.Debugf("pod statuses could be fetched this time due to %s", err)
	}
}

func (r *Rollout) String() string {
	if r.namespace == "default" {
		return fmt.Sprintf("%s/%s", r.rType, r.name)
	}

	return fmt.Sprintf("%s:%s/%s", r.namespace, r.rType, r.name)
}

func (r *Rollout) Name() string {
	return r.name
}

func (r *Rollout) Status() Status {
	return r.status
}

func (r *Rollout) IsStatusCheckCompleteOrCancelled() bool {
	return r.done || r.statusCode == proto.StatusCode_STATUSCHECK_USER_CANCELLED
}

func (r *Rollout) StatusMessage() string {
	for _, p := range r.pods {
		if s := p.ActionableError(); s.ErrCode != proto.StatusCode_STATUSCHECK_SUCCESS {
			return fmt.Sprintf("%s\n", s.Message)
		}
	}
	return r.status.String()
}

func (r *Rollout) MarkComplete() {
	r.done = true
}

// ReportSinceLastUpdated returns a string representing rollout status along with tab header
// e.g.
//  - testNs:deployment/leeroy-app: waiting for rollout to complete. (1/2) pending
//      - testNs:pod/leeroy-app-xvbg : error pulling container image
func (r *Rollout) ReportSinceLastUpdated(isMuted bool) string {
	if r.status.reported && !r.status.changed {
		return ""
	}
	r.status.reported = true
	if r.status.String() == "" {
		return ""
	}
	var result strings.Builder
	// Pod container statuses can be empty.
	// This can happen when
	// 1. No pods have been scheduled for the rollout
	// 2. All containers are in running phase with no errors.
	// In such case, avoid printing any status update for the rollout.
	for _, p := range r.pods {
		if s := p.ActionableError().Message; s != "" {
			result.WriteString(fmt.Sprintf("%s %s %s: %s\n", tab, tabHeader, p, s))
			// if logs are muted, write container logs to file and last 3 lines to
			// result.
			out, writeTrimLines, err := withLogFile(p.Name(), &result, p.Logs(), isMuted)
			if err != nil {
				logrus.Debugf("could not create log file %v", err)
			}
			trimLines := []string{}
			for i, l := range p.Logs() {
				formattedLine := fmt.Sprintf("%s %s > %s\n", tab, tab, strings.TrimSuffix(l, "\n"))
				if isMuted && i >= len(p.Logs())-maxLogLines {
					trimLines = append(trimLines, formattedLine)
				}
				out.Write([]byte(formattedLine))
			}
			writeTrimLines(trimLines)
		}
	}
	return fmt.Sprintf("%s %s: %s%s", tabHeader, r, r.StatusMessage(), result.String())
}

func (r *Rollout) cleanupStatus(msg string) string {
	clean := strings.ReplaceAll(msg, fmt.Sprintf("%s %q", r.rType, r.Name()), "")
	if len(clean) > 0 {
		clean = strings.ToLower(clean[0:1]) + clean[1:]
	}
	return clean
}

// parses out connection error
// $kubectl logs somePod -f
// Unable to connect to the server: dial tcp x.x.x.x:443: connect: network is unreachable

// Parses out errors when kubectl was killed on client side
// $kubectl logs testPod  -f
// 2020/06/18 17:28:31 service is running
// Killed: 9
func parseKubectlRolloutError(details string, err error) proto.ActionableErr {
	switch {
	case err == nil && strings.Contains(details, rollOutSuccess):
		return proto.ActionableErr{
			ErrCode: proto.StatusCode_STATUSCHECK_SUCCESS,
			Message: details,
		}
	case err == nil:
		return proto.ActionableErr{
			ErrCode: proto.StatusCode_STATUSCHECK_DEPLOYMENT_ROLLOUT_PENDING,
			Message: details,
		}
	case strings.Contains(err.Error(), connectionErrMsg):
		return proto.ActionableErr{
			ErrCode: proto.StatusCode_STATUSCHECK_KUBECTL_CONNECTION_ERR,
			Message: MsgKubectlConnection,
		}
	case strings.Contains(err.Error(), killedErrMsg):
		return proto.ActionableErr{
			ErrCode: proto.StatusCode_STATUSCHECK_KUBECTL_PID_KILLED,
			Message: msgKubectlKilled,
		}
	default:
		return proto.ActionableErr{
			ErrCode: proto.StatusCode_STATUSCHECK_UNKNOWN,
			Message: err.Error(),
		}
	}
}

func isErrAndNotRetryAble(statusCode proto.StatusCode) bool {
	return statusCode != proto.StatusCode_STATUSCHECK_KUBECTL_CONNECTION_ERR &&
		statusCode != proto.StatusCode_STATUSCHECK_DEPLOYMENT_ROLLOUT_PENDING
}

// HasEncounteredUnrecoverableError goes through all pod statuses and return true
// if any cannot be recovered
func (r *Rollout) HasEncounteredUnrecoverableError() bool {
	for _, p := range r.pods {
		if _, ok := nonRetryContainerErrors[p.ActionableError().ErrCode]; ok {
			return true
		}
	}
	return false
}

func (r *Rollout) fetchPods(ctx context.Context) error {
	timeoutContext, cancel := context.WithTimeout(ctx, defaultPodCheckDeadline)
	defer cancel()
	pods, err := r.podValidator.Run(timeoutContext)
	if err != nil {
		return err
	}

	newPods := map[string]validator.Resource{}
	r.status.changed = false
	for _, p := range pods {
		originalPod, found := r.pods[p.String()]
		if !found || originalPod.StatusUpdated(p) {
			r.status.changed = true
			switch p.ActionableError().ErrCode {
			case proto.StatusCode_STATUSCHECK_CONTAINER_CREATING,
				proto.StatusCode_STATUSCHECK_POD_INITIALIZING:
				event.ResourceStatusCheckEventUpdated(p.String(), p.ActionableError())
			default:
				event.ResourceStatusCheckEventCompleted(p.String(), p.ActionableError())
			}
		}
		newPods[p.String()] = p
	}
	r.pods = newPods
	return nil
}

// StatusCode() returns the rollout status code if the status check is cancelled
// or if no pod data exists for this rollout.
// If pods are fetched, this function returns the error code a pod container encountered.
func (r *Rollout) StatusCode() proto.StatusCode {
	// do not process pod status codes if another rollout failed
	// or the user aborted the run.
	if r.statusCode == proto.StatusCode_STATUSCHECK_USER_CANCELLED {
		return r.statusCode
	}
	for _, p := range r.pods {
		if s := p.ActionableError().ErrCode; s != proto.StatusCode_STATUSCHECK_SUCCESS {
			return s
		}
	}
	return r.statusCode
}

func (r *Rollout) WithPodStatuses(scs []proto.StatusCode) *Rollout {
	r.pods = map[string]validator.Resource{}
	for i, s := range scs {
		name := fmt.Sprintf("%s-%d", r.name, i)
		r.pods[name] = validator.NewResource("test", "pod", "foo", validator.Status("failed"),
			proto.ActionableErr{Message: "pod failed", ErrCode: s}, nil)
	}
	return r
}
