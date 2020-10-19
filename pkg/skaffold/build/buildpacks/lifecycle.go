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

package buildpacks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	lifecycle "github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/pack"
	"github.com/buildpacks/pack/project"
	"github.com/sirupsen/logrus"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

// For testing
var (
	runPackBuildFunc = runPackBuild
)

// images is a global list of builder/runner image pairs that are already pulled.
// In a skaffold session, typically a skaffold dev loop, we want to avoid asking `pack`
// to pull the images that are already pulled.
var images pulledImages

func (b *Builder) build(ctx context.Context, out io.Writer, a *latest.Artifact, tag string, r ArtifactResolver) (string, error) {
	artifact := a.BuildpackArtifact
	workspace := a.Workspace

	// Read `project.toml` if it exists.
	path := filepath.Join(workspace, artifact.ProjectDescriptor)
	projectDescriptor, err := project.ReadProjectDescriptor(path)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read project descriptor %q: %w", path, err)
	}

	// To improve caching, we always build the image with [:latest] tag
	// This way, the lifecycle is able to "bootstrap" from the previously built image.
	// The image will then be tagged as usual with the tag provided by the tag policy.
	parsed, err := docker.ParseReference(tag)
	if err != nil {
		return "", fmt.Errorf("parsing tag %q: %w", tag, err)
	}
	latest := parsed.BaseName + ":latest"

	// Evaluate Env Vars.
	env, err := env(a, b.mode, projectDescriptor)
	if err != nil {
		return "", fmt.Errorf("unable to evaluate env variables: %w", err)
	}
	// List buildpacks to be used for the build.
	// Those specified in the skaffold.yaml replace those in the project.toml.
	buildpacks := artifact.Buildpacks
	if len(buildpacks) == 0 {
		for _, bp := range projectDescriptor.Build.Buildpacks {
			if bp.ID != "" {
				if bp.Version == "" {
					buildpacks = append(buildpacks, bp.ID)
				} else {
					buildpacks = append(buildpacks, fmt.Sprintf("%s@%s", bp.ID, bp.Version))
				}
				// } else {
				// TODO(dgageot): Support URI.
			}
		}
	}

	builderImage, runImage, noPull, err := resolveArtifactDependencies(artifact, r, a.Dependencies)
	if err != nil {
		return "", fmt.Errorf("unable to resolve required artifacts: %w", err)
	}
	// Does the builder image need to be pulled?
	if noPull == false {
		noPull = images.AreAlreadyPulled(builderImage, runImage)
	}

	if err := runPackBuildFunc(ctx, out, b.localDocker, pack.BuildOptions{
		AppPath:      workspace,
		Builder:      builderImage,
		RunImage:     runImage,
		Buildpacks:   buildpacks,
		Env:          env,
		Image:        latest,
		NoPull:       noPull,
		TrustBuilder: artifact.TrustBuilder,
		// TODO(dgageot): Support project.toml include/exclude.
		// FileFilter: func(string) bool { return true },
	}); err != nil {
		return "", err
	}

	images.MarkAsPulled(builderImage, runImage)

	return latest, nil
}

func resolveArtifactDependencies(artifact *latest.BuildpackArtifact, r ArtifactResolver, deps []*latest.ArtifactDependency) (builderImage string, runImage string, noPull bool, err error) {
	builderImage, runImage = artifact.Builder, artifact.RunImage
	for _, d := range deps {
		if builderImage == d.Alias {
			builderImage, err = r.GetImageTag(d.ImageName)
			if err != nil {
				return
			}
			noPull = true
		}
		if runImage == d.Alias {
			runImage, err = r.GetImageTag(d.ImageName)
			if err != nil {
				return
			}
			noPull = true
		}
	}

	if noPull == true {
		logrus.Warnln("Disabled remote image pull for buildpacks since one of builder or run image is local.")
	}
	return
}

func runPackBuild(ctx context.Context, out io.Writer, localDocker docker.LocalDaemon, opts pack.BuildOptions) error {
	packClient, err := pack.NewClient(
		pack.WithDockerClient(localDocker.RawClient()),
		pack.WithLogger(NewLogger(out)),
		pack.WithFetcher(newFetcher(out, localDocker)),
	)
	if err != nil {
		return fmt.Errorf("unable to create pack client: %w", err)
	}

	err = packClient.Build(ctx, opts)
	// pack turns exit codes from the lifecycle into `failed with status code: N`
	if err != nil {
		err = rewriteLifecycleStatusCode(err)
	}
	return err
}

func rewriteLifecycleStatusCode(lce error) error {
	prefix := "failed with status code: "
	lceText := lce.Error()
	if strings.HasPrefix(lceText, prefix) {
		sc := lceText[len(prefix):]
		if code, err := strconv.Atoi(sc); err == nil {
			return errors.New(mapLifecycleStatusCode(code))
		}
	}
	return lce
}

func mapLifecycleStatusCode(code int) string {
	switch code {
	case lifecycle.CodeInvalidArgs:
		return "lifecycle reported invalid arguments"
	case lifecycle.CodeFailedDetect:
		return "buildpacks could not determine application type"
	case lifecycle.CodeFailedBuild:
		return "buildpacks failed to build"
	case lifecycle.CodeFailedSave:
		return "buildpacks failed to save image"
	case lifecycle.CodeIncompatible:
		return "incompatible lifecycle version"
	default:
		return fmt.Sprintf("lifecycle failed with status code %d", code)
	}
}

func envMap(env []string) map[string]string {
	kv := make(map[string]string)

	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		kv[parts[0]] = parts[1]
	}

	return kv
}
