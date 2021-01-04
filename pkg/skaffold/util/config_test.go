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

package util

import (
	"os"
	"testing"

	"github.com/GoogleContainerTools/skaffold/testutil"
)

func TestReadConfiguration(t *testing.T) {
	testutil.Run(t, "", func(t *testutil.T) {
		t.NewTempDir().
			Write("skaffold.yaml", "some yaml").
			Chdir()
		f, err := NewConfigFile("skaffold.yaml")
		t.CheckNoError(err)
		content, err := f.Read()
		t.CheckNoError(err)
		t.CheckDeepEqual([]byte("some yaml"), content)
		t.CheckDeepEqual(f.Dir(), ".")
	})
}

func TestReadConfigurationFallback(t *testing.T) {
	testutil.Run(t, "", func(t *testutil.T) {
		t.NewTempDir().
			// skaffold.yaml doesn't exist but .yml does
			Write("skaffold.yml", "some yaml").
			Chdir()

		f, err := NewConfigFile("skaffold.yaml")
		t.CheckNoError(err)
		content, err := f.Read()
		t.CheckNoError(err)
		t.CheckDeepEqual([]byte("some yaml"), content)
		t.CheckDeepEqual(f.Dir(), ".")
	})
}

func TestReadConfigurationNotFound(t *testing.T) {
	_, err := NewConfigFile("unknown-config-file.yaml")
	testutil.CheckError(t, true, err)
	if !os.IsNotExist(err) {
		t.Error("error should say that file doesn't exist")
	}
}

func TestReadConfigurationRemote(t *testing.T) {
	remoteFile := testutil.ServeFile(t, []byte("remote file"))
	f, err := NewConfigFile(remoteFile)
	testutil.CheckError(t, false, err)
	content, err := f.Read()

	testutil.CheckErrorAndDeepEqual(t, false, err, []byte("remote file"), content)
}
