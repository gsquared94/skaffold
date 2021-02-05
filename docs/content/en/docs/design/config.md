---
title: "Skaffold Pipeline"
linkTitle: "Skaffold Pipeline"
weight: 40
aliases: [/docs/concepts/config]
---

You can configure Skaffold with the Skaffold configuration file,
`skaffold.yaml`. The configuration file should be placed in the root of your
project directory; when you run the `skaffold` command, Skaffold will try to
read the configuration file from the current directory.

`skaffold.yaml` consists of several different components:

| Component  | Description |
| ---------- | ------------|
| `apiVersion` | The Skaffold API version you would like to use. The current API version is {{< skaffold-version >}}. |
| `kind`  |  The Skaffold configuration file has the kind `Config`.  |
| `metadata`  |  Holds additional properties like the `name` of this configuration.  |
| `build`  |  Specifies how Skaffold builds artifacts. You have control over what tool Skaffold can use, how Skaffold tags artifacts and how Skaffold pushes artifacts. Skaffold supports using local Docker daemon, Google Cloud Build, Kaniko, or Bazel to build artifacts. See [Builders](/docs/pipeline-stages/builders) and [Taggers]({{< relref "/docs/pipeline-stages/taggers" >}}) for more information. |
| `test` |  Specifies how Skaffold tests artifacts. Skaffold supports [container-structure-tests](https://github.com/GoogleContainerTools/container-structure-test) to test built artifacts. See [Testers]({{< relref "/docs/pipeline-stages/testers" >}}) for more information. |
| `deploy` |  Specifies how Skaffold deploys artifacts. Skaffold supports using `kubectl`, `helm`, or `kustomize` to deploy artifacts. See [Deployers]({{< relref "/docs/pipeline-stages/deployers" >}}) for more information. |
| `profiles`|  Profile is a set of settings that, when activated, overrides the current configuration. You can use Profile to override the `build`, `test` and `deploy` sections. |
| `requires`|  Specifies a list of other skaffold configurations to import into the current config |

You can [learn more]({{< relref "/docs/references/yaml" >}}) about the syntax of `skaffold.yaml`.

## Configuration dependencies

In addition to authoring pipelines in a skaffold configuration file, we can also import pipelines from other existing configurations as dependencies. Skaffold manages all imported and defined pipelines in the same session. It also ensures all artifacts in a required configs are built prior to those in current config (provided the artifacts have dependencies defined); and all deploys in required configs are applied prior to those in current config.

### Local config dependency

Consider a `skaffold.yaml` defined as:
```yaml
apiVersion: skaffold/vX
kind: Config
metadata:
  name: cfg1
build:
  # build definition
deploy:
  # deploy definition

---

apiVersion: skaffold/vX
kind: Config
metadata:
  name: cfg2
build:
  # build definition
deploy:
  # deploy definition
```

Configurations `cfg1` and `cfg2` from the above file can be imported as dependencies in your current config, via:

```yaml
apiVersion: skaffold/v2beta11
kind: Config
requires:
  - configs: ["cfg1", "cfg2"]
    path: path/to/other/skaffold.yaml 
build:
  # build definition
deploy:
  # deploy definition
```

If the `configs` list isn't defined then it imports all the configs defined in the file pointed by `path`. Additionally, if the `path` to the configuration isn't defined it assumes that all the required configs are defined in the same file as the current config.
### Remote config dependency

The required skaffold config can live in a remote git repository:

```yaml
apiVersion: skaffold/v2beta12
kind: Config
requires:
  - configs: ["cfg1", "cfg2"]
    git:
      repo: http://github.com/GoogleContainerTools/skaffold.git
      path: getting-started/skaffold.yaml
      ref: master
```

The environment variable `SKAFFOLD_REMOTE_CACHE_DIR` or flag `--remote-cache-dir` specifies the download location for all remote repos. If undefined then it defaults to `~/.skaffold/repos`. 
The repo root directory name is a hash of the repo `uri` and the `branch/ref`.
Every execution of a remote module resets the cached repo to the referenced ref. The default ref is master. If master is not defined then it defaults to main.
The remote config gets treated like a local config after substituting the path with the actual path in the cache directory.
  
### Profile Activation in required configs

Additionally the `activeProfiles` stanza can define the profiles to be activated in the required configs, via:

```yaml
apiVersion: skaffold/v2beta11
kind: Config
metadata:
    name: cfg
requires:
  - path: ./path/to/required/skaffold.yaml
    configs: [cfg1, cfg2]                 
    activeProfiles:                                     
     - name: profile1                               
       activatedBy: [profile2, profile3] 
```

Here, `profile1` is a profile that needs to exist in both configs `cfg1` and `cfg2`; while `profile2` and `profile3` are profiles defined in the current config `cfg`. If the current config is activated with either `profile2` or `profile3` then the required configs `cfg1` and `cfg2` are imported with `profile1` applied. If the `activatedBy` clause is omitted then that `profile1` always gets applied for the imported configs.