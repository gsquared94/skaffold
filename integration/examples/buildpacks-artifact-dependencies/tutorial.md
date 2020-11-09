# Using custom run image for CNB builder


## Introduction

### What is CNB?
[Cloud Native Buildpacks](https://buildpacks.io/) (CNB) enable building
a container image from source code without the need for a Dockerfile.
Skaffold supports building with CNB, requiring only
a local Docker daemon. 

Buildpacks uses two images when building an application image:
  - A _run image_ serves as the base image for the resulting application image.
  - A _build image_ acts as the host for performing the build.

### What you'll learn

- how we can annotate an arbitrary image and use it as a CNB 
[run image](https://buildpacks.io/docs/concepts/components/stack/).
- how we can easily integrate the custom run image into our Buildpacks application using Skaffold's *artifact dependencies* feature.

**Time to complete**: About 10 minutes

Click the **Start** button to move to the next step.

## First steps

### Install Skaffold

We'll manually install the bleeding-edge version of `Skaffold` to use the pre-release feature of `artifact dependencies`. 

Run:
```bash
curl -Lo skaffold https://storage.googleapis.com/skaffold/builds/latest/skaffold-linux-amd64
sudo install skaffold /usr/bin/skaffold
```

## Congratulations

<walkthrough-conclusion-trophy></walkthrough-conclusion-trophy>

You’re all set!

You can now have users launch your tutorial in Cloud Shell and have them start using your project with ease.


