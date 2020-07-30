/*
Copyright 2020 The Skaffold Authors

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

package build

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/logfile"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

// WithLogFile wraps an `artifactBuilder` so that it optionally outputs its logs to a file.
func WithLogFile(builder ArtifactBuilder, suppressedLogs []string) ArtifactBuilder {
	// TODO(dgageot): this should probably be moved somewhere else.
	if !(util.StrSliceContains(suppressedLogs, "build") || util.StrSliceContains(suppressedLogs, "all")) {
		return builder
	}

	return func(ctx context.Context, out io.Writer, artifact *latest.Artifact, opts BuilderOptions) (string, error) {
		file, err := logfile.Create(artifact.ImageName + ".log")
		if err != nil {
			return "", fmt.Errorf("unable to create log file for %s: %w", artifact.ImageName, err)
		}
		fmt.Fprintln(out, " - writing logs to", file.Name())

		// Print logs to a memory buffer and to a file.
		var buf bytes.Buffer
		w := io.MultiWriter(file, &buf)

		// Run the build.
		digest, err := builder(ctx, w, artifact, opts)

		// After the build finishes, close the log file. If the build failed, print the full log to the console.
		file.Close()
		if err != nil {
			buf.WriteTo(out)
		}

		return digest, err
	}
}
