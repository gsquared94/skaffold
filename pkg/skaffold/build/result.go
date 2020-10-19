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
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

const bufferedLinesPerArtifact = 10000

// For testing
var (
	buffSize = bufferedLinesPerArtifact
)

// logAggregator provides an interface to create an output writer for each artifact build and later aggregate the logs in build order.
// The order of output is not guaranteed between multiple builds running concurrently.
type logAggregator interface {
	// GetWriter returns an output writer tracked by the logAggregator
	GetWriter() (io.WriteCloser, error)
	// PrintInOrder prints the output from each allotted writer in build order.
	// It blocks until the instantiated capacity of io writers have been all allotted and closed, or the context is cancelled.
	PrintInOrder(ctx context.Context, out io.Writer)
}

type logAggregatorImpl struct {
	messages   chan chan string
	size       int
	capacity   int
	countMutex sync.Mutex
}

func (l *logAggregatorImpl) GetWriter() (io.WriteCloser, error) {
	if err := l.checkCapacity(); err != nil {
		return nil, err
	}
	r, w := io.Pipe()
	ch := make(chan string, buffSize)
	l.messages <- ch
	// write the build output to a buffered channel.
	go l.writeToChannel(r, ch)
	return w, nil
}

func (l *logAggregatorImpl) PrintInOrder(ctx context.Context, out io.Writer) {
	go func() {
		<-ctx.Done()
		// we handle cancellation by passing a nil struct instead of closing the channel.
		// This makes it easier to flush all pending messages on the buffered channel before returning and avoid any race with pending requests for new writers.
		l.messages <- nil
	}()
	for i := 0; i < l.capacity; i++ {
		ch := <-l.messages
		if ch == nil {
			return
		}
		// read from each build's message channel and write to the given output.
		printResult(out, ch)
	}
}

func (l *logAggregatorImpl) checkCapacity() error {
	l.countMutex.Lock()
	defer l.countMutex.Unlock()
	if l.size == l.capacity {
		return fmt.Errorf("failed to create writer: capacity exceeded")
	}
	l.size++
	return nil
}

func printResult(out io.Writer, output chan string) {
	for line := range output {
		fmt.Fprintln(out, line)
	}
}

func (l *logAggregatorImpl) writeToChannel(r io.Reader, lines chan string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	close(lines)
}

func newLogAggregator(capacity int) logAggregator {
	return &logAggregatorImpl{capacity: capacity, messages: make(chan chan string, capacity)}
}

// BuiltArtifacts stores the results of each artifact build.
type BuiltArtifacts interface {
	Record(a *latest.Artifact, tag string)
	GetImageTag(imageName string) (string, error)
	GetArtifacts(s []*latest.Artifact) ([]Artifact, error)
}

func NewBuiltArtifacts(prebuilt []Artifact) BuiltArtifacts {
	ba := &builtArtifactsImpl{m: new(sync.Map)}
	for _, a := range prebuilt {
		ba.m.Store(a.ImageName, a.Tag)
	}
	return ba
}

type builtArtifactsImpl struct {
	m *sync.Map
}

func (ba *builtArtifactsImpl) Record(a *latest.Artifact, tag string) {
	ba.m.Store(a.ImageName, tag)
}

func (ba *builtArtifactsImpl) GetImageTag(imageName string) (string, error) {
	v, ok := ba.m.Load(imageName)
	if !ok {
		return "", fmt.Errorf("could not find build result for image %s", imageName)
	}
	t, ok := v.(string)
	if !ok {
		logrus.Fatalf("invalid build output recorded for image %s", imageName)
	}
	return t, nil
}

func (ba *builtArtifactsImpl) GetArtifacts(s []*latest.Artifact) ([]Artifact, error) {
	var builds []Artifact
	for _, a := range s {
		t, err := ba.GetImageTag(a.ImageName)
		if err != nil {
			return nil, err
		}
		builds = append(builds, Artifact{ImageName: a.ImageName, Tag: t})
	}
	return builds, nil
}
