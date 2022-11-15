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

import "os"

// for testing
var lookupEnv = os.LookupEnv

// CICDPlatform denotes a known CI-CD platform like Github Actions, Azure Pipelines, etc that the current skaffold run is being executed in.
type CICDPlatform struct {
	// Name denotes the name of the CI-CD platform
	Name string
	// IdentifierEnv denotes the set of known environment variables for this CI-CD platform
	IdentifierEnv []string
}

// IsActive returns true if the current CI-CD platform is detected
func (p CICDPlatform) IsActive() bool {
	for _, e := range p.IdentifierEnv {
		if _, ok := lookupEnv(e); ok {
			return true
		}
	}
	return false
}

// GetCICDPlatform returns the active CICD platform.
func GetCICDPlatform() string {
	for _, p := range CICDPlatforms {
		if p.IsActive() {
			return p.Name
		}
	}
	return ""
}

var CICDPlatforms = []CICDPlatform{
	{Name: "azure-pipelines", IdentifierEnv: []string{"TF_BUILD"}},
	{Name: "bamboo", IdentifierEnv: []string{"bamboo_buildKey"}},
	{Name: "buildkite", IdentifierEnv: []string{"BUILDKITE"}},
	{Name: "circle-ci", IdentifierEnv: []string{"CIRCLECI"}},
	{Name: "cirrus-ci", IdentifierEnv: []string{"CIRRUS_CI"}},
	{Name: "code-build", IdentifierEnv: []string{"CODEBUILD_BUILD_ID"}},
	{Name: "github-actions", IdentifierEnv: []string{"GITHUB_ACTIONS"}},
	{Name: "gitlab-ci", IdentifierEnv: []string{"GITLAB_CI"}},
	{Name: "heroku-ci", IdentifierEnv: []string{"HEROKU_TEST_RUN_ID"}},
	{Name: "jenkins", IdentifierEnv: []string{"JENKINS_URL"}},
	{Name: "team-city", IdentifierEnv: []string{"TEAMCITY_VERSION"}},
	{Name: "travis-ci", IdentifierEnv: []string{"TRAVIS"}},
	{Name: "unknown", IdentifierEnv: []string{"BUILD_ID", "BUILD_NUMBER", "CI"}},
}
