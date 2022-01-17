/*
Copyright 2021 The Skaffold Authors

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

package platform

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/platforms"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	coreV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	kubernetesclient "github.com/GoogleContainerTools/skaffold/pkg/skaffold/kubernetes/client"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/output/log"
	latestV1 "github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest/v1"
)

type Resolver struct {
	platformsByImageName map[string]Platforms
}

func (r Resolver) GetPlatforms(imageName string) Platforms {
	return r.platformsByImageName[imageName]
}

func NewResolver(ctx context.Context, pipelines []latestV1.Pipeline, cliPlatformsSelection []string, runMode config.RunMode, kubeContext string) (Resolver, error) {
	r := Resolver{}
	r.platformsByImageName = make(map[string]Platforms)

	var fromCli, fromClusterNodes Platforms
	var err error
	fromCli, err = ParsePlatforms(cliPlatformsSelection)
	if err != nil {
		return r, fmt.Errorf("failed to parse platforms: %w", err)
	}
	log.Entry(ctx).Debugf("CLI platforms provided: %q", fromCli)

	if runMode == config.RunModes.Dev || runMode == config.RunModes.Debug || runMode == config.RunModes.Run {
		fromClusterNodes, err = GetClusterPlatforms(ctx, kubeContext, runMode == config.RunModes.Dev || runMode == config.RunModes.Debug)
		if err != nil {
			log.Entry(ctx).Debugf("failed to get cluster node details: %w", err)
			log.Entry(ctx).Warnln("failed to detect active kubernetes cluter node platform. Specify the correct build platform in the `skaffold.yaml` file or using the `--platform` flag")
		}
	}
	log.Entry(ctx).Debugf("platforms detected from active kubernetes cluster nodes: %q", fromClusterNodes)

	for _, pipeline := range pipelines {
		platforms := fromCli
		if len(platforms) == 0 {
			platforms, err = ParsePlatforms(pipeline.Build.Platforms)
			if err != nil {
				return r, fmt.Errorf("failed to parse platforms: %w", err)
			}
		}
		if p := platforms; len(fromClusterNodes) != 0 {
			platforms = platforms.Intersect(fromClusterNodes)
			if len(platforms) == 0 && len(p) != 0 {
				return r, fmt.Errorf("build target platforms %q do not match active kubernetes cluster node platforms %q", p, fromClusterNodes)
			}
		}
		for _, artifact := range pipeline.Build.Artifacts {
			pl := platforms
			if len(platforms) != 0 && len(artifact.Platforms) != 0 {
				constraints, err := ParsePlatforms(artifact.Platforms)
				if err != nil {
					return r, fmt.Errorf("failed to parse platforms: %w", err)
				}
				pl = platforms.Intersect(constraints)
				if len(pl) == 0 {
					return r, fmt.Errorf("build target platforms %q do not match platform constraints %q defined for artifact %q", platforms, artifact.Platforms, artifact.ImageName)
				}
			}
			r.platformsByImageName[artifact.ImageName] = pl
			log.Entry(ctx).Debugf("platforms selected for artifact %q: %q", artifact.ImageName, pl)
		}
	}
	return r, nil
}

func ParsePlatforms(ps []string) (Platforms, error) {
	var platforms Platforms
	for _, p := range ps {
		platform, err := Parse(p)
		if err != nil {
			return nil, UnknownPlatformCLIFlag(p, err)
		}
		platforms = append(platforms, platform)
	}
	return platforms, nil
}

// GetClusterPlatforms returns the platforms for the active kubernetes cluster.
// For `dev` or `debug` if there are multiple cluster node platforms, then it returns only one selection, preferably matching the host platform.
func GetClusterPlatforms(ctx context.Context, kContext string, isDevOrDebug bool) (Platforms, error) {
	client, err := kubernetesclient.Client(kContext)
	if err != nil {
		return nil, fmt.Errorf("failed to determine kubernetes cluster node platforms: %w", err)
	}
	nodes, err := client.CoreV1().Nodes().List(ctx, coreV1.ListOptions{})
	if nodes == nil || err != nil {
		return nil, fmt.Errorf("failed to determine kubernetes cluster node platforms: %w", err)
	}
	set := make(map[string]Platform)
	for _, n := range nodes.Items {
		pl := Platform{Platform: v1.Platform{
			Architecture: n.Status.NodeInfo.Architecture,
			OS:           n.Status.NodeInfo.OperatingSystem,
		}}
		set[pl.String()] = pl
	}

	var ps Platforms
	for _, v := range set {
		ps = append(ps, v)
	}

	if !isDevOrDebug || len(ps) <= 1 {
		return ps, nil
	}

	// If there's more than 1 node platform type and we're running `dev` or `debug`,
	// then select the platform type that matches the host platform.
	// If there's no match then just return the first node platform type found.
	hostPlatform := Platform{Platform: platforms.DefaultSpec()}

	filtered := ps.Intersect(Platforms{hostPlatform})
	if len(filtered) == 0 {
		return ps[0:1], nil
	}
	return filtered, nil
}
