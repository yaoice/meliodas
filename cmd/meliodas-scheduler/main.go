/*
Copyright 2020 The Kubernetes Authors.

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

package main

import (
	"math/rand"
	"os"
	"time"

	"k8s.io/kubernetes/cmd/kube-scheduler/app"

	"github.com/yaoice/meliodas/pkg/qos"
	"github.com/yaoice/meliodas/pkg/scheduling"
)

func main() {
	rand.Seed(time.Now().UnixNano())
	// Register custom plugins to the scheduler framework.
	// Later they can consist of scheduler profile(s) and hence
	// used by various kinds of workloads.
	command := app.NewSchedulerCommand(
		app.WithPlugin(scheduling.Name, scheduling.New),
		app.WithPlugin(qos.Name, qos.New),
	)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
