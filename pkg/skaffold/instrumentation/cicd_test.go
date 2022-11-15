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

package instrumentation

import (
	"testing"

	"github.com/GoogleContainerTools/skaffold/testutil"
)

func TestGetCICDPlatform(t *testing.T) {
	tests := []struct {
		description string
		env         map[string]string
		expected    string
	}{
		{
			description: "azure-pipelines",
			env: map[string]string{
				"TF_BUILD": "1",
			},
			expected: "azure-pipelines",
		},
		{
			description: "bamboo",
			env: map[string]string{
				"bamboo_buildKey": "1",
			},
			expected: "bamboo",
		},
		{
			description: "buildkite",
			env: map[string]string{
				"BUILDKITE": "1",
			},
			expected: "buildkite",
		},
		{
			description: "circle-ci",
			env: map[string]string{
				"CIRCLECI": "1",
			},
			expected: "circle-ci",
		},
		{
			description: "cirrus-ci",
			env: map[string]string{
				"CIRRUS_CI": "1",
			},
			expected: "cirrus-ci",
		},
		{
			description: "code-build",
			env: map[string]string{
				"CODEBUILD_BUILD_ID": "1",
			},
			expected: "code-build",
		},
		{
			description: "github-actions",
			env: map[string]string{
				"GITHUB_ACTIONS": "1",
			},
			expected: "github-actions",
		},
		{
			description: "gitlab-ci",
			env: map[string]string{
				"GITLAB_CI": "1",
			},
			expected: "gitlab-ci",
		},
		{
			description: "heroku-ci",
			env: map[string]string{
				"HEROKU_TEST_RUN_ID": "1",
			},
			expected: "heroku-ci",
		},
		{
			description: "jenkins",
			env: map[string]string{
				"JENKINS_URL": "1",
			},
			expected: "jenkins",
		},
		{
			description: "team-city",
			env: map[string]string{
				"TEAMCITY_VERSION": "1",
			},
			expected: "team-city",
		},
		{
			description: "travis-ci",
			env: map[string]string{
				"TRAVIS": "1",
			},
			expected: "travis-ci",
		},
		{
			description: "unknown",
			env: map[string]string{
				"BUILD_ID": "1",
			},
			expected: "unknown",
		},
		{
			description: "unknown",
			env: map[string]string{
				"BUILD_NUMBER": "1",
			},
			expected: "unknown",
		},
		{
			description: "unknown",
			env: map[string]string{
				"CI": "1",
			},
			expected: "unknown",
		},
		{
			description: "not ci",
			env:         map[string]string{},
			expected:    "",
		},
	}

	for _, test := range tests {
		testutil.Run(t, test.description, func(t *testutil.T) {
			t.Override(&lookupEnv, func(key string) (string, bool) {
				if val, ok := test.env[key]; ok {
					return val, true
				}
				return "", false
			})
			t.CheckDeepEqual(test.expected, GetCICDPlatform())
		})
	}
}
