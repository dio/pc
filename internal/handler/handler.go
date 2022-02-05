// Copyright 2022 Dhi Aurrahman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"sigs.k8s.io/yaml"
)

//go:embed template/config.yaml
var config string

func New(ctx context.Context, args []string) (*Handler, func(error)) {
	if len(args) > 1 {
		// Strip out "func-e run".
		if args[0] == "func-e" && args[1] == "run" {
			args = args[2:]
		}
	}

	// Find for -c <string> or --config-path <string>. TODO(dio): Handle --config-yaml.
	var configFile string
	var index int
	for index, arg := range args {
		if arg == "-c" || arg == "--config-path" {
			configFile = args[index+1]
			break
		}
	}

	dir, err := ioutil.TempDir("", "pc_")
	if err != nil {
		fmt.Printf("failed to create a temporary directory: %v\n", err)
		os.Exit(1)
	}

	if configFile == "" {
		fmt.Printf("config file is required, set --config-path or -c to /path/to/your/config/file.yaml\n")
		os.Exit(1)
	}

	built, err := buildConfig(dir, configFile)
	if err != nil {
		fmt.Printf("failed to build config to watch in %s: %v\n", dir, err)
		os.Exit(1)
	}

	if len(args) >= index+1 {
		// index+1 is the index of the --config-path or -c value.
		args[index+1] = built.Config
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Printf("failed to create watcher: %v\n", err)
		os.Exit(1)
	}

	return &Handler{
			ctx:         ctx,
			watcher:     watcher,
			configFile:  configFile,
			configFiles: built,
			dir:         dir,
			args:        args,
		}, func(error) {
			if watcher != nil {
				watcher.Close()
			}
			os.RemoveAll(dir)
		}
}

type Handler struct {
	ctx         context.Context
	watcher     *fsnotify.Watcher
	configFile  string
	configFiles ConfigFiles
	dir         string
	args        []string
}

func (h *Handler) Run() error {
	err := h.watcher.Add(h.configFile)
	if err != nil {
		return err
	}

	for {
		select {
		case <-h.ctx.Done():
			return nil
		case event, ok := <-h.watcher.Events:
			if !ok {
				continue
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				_, err := buildConfig(h.dir, h.configFile)
				if err != nil {
					fmt.Println(err)
				}
			}
		case err := <-h.watcher.Errors:
			fmt.Println(err)
		}
	}
}

func (h *Handler) Args() []string {
	if !contains(h.args, "--admin-address-path") {
		h.args = append(h.args, "--admin-address-path", h.configFiles.AdminAddressPath)
	}
	if !contains(h.args, "--use-dynamic-base-id") || !contains(h.args, "--base-id") {
		// The server chooses a base ID dynamically. Supersedes a static base ID. May not be used when
		// the restart epoch is non-zero.
		h.args = append(h.args, "--use-dynamic-base-id") // So we can run multiple proxies.
	}
	return h.args
}

func (h *Handler) Files() ConfigFiles {
	return h.configFiles
}

type ConfigFiles struct {
	Clusters         string
	Listeners        string
	Config           string
	AdminAddressPath string
}

func buildConfig(dir, configFile string) (ConfigFiles, error) {
	content, err := os.ReadFile(configFile)
	if err != nil {
		return ConfigFiles{}, err
	}

	j, err := yaml.YAMLToJSON(content)
	if err != nil {
		return ConfigFiles{}, err
	}

	var bootstrap bootstrapv3.Bootstrap
	err = protojson.Unmarshal(j, &bootstrap)
	if err != nil {
		return ConfigFiles{}, err
	}

	err = bootstrap.ValidateAll()
	if err != nil {
		return ConfigFiles{}, err
	}

	data := ConfigFiles{
		Clusters:         filepath.Join(dir, "clusters.json"),
		Listeners:        filepath.Join(dir, "listeners.json"),
		Config:           filepath.Join(dir, "config.yaml"),
		AdminAddressPath: filepath.Join(dir, "admin.txt"),
	}

	err = writeDynamicConfig(data.Clusters, clustersToMessages(bootstrap.StaticResources.Clusters))
	if err != nil {
		return ConfigFiles{}, err
	}

	err = writeDynamicConfig(data.Listeners, listenersToMessages(bootstrap.StaticResources.Listeners))
	if err != nil {
		return ConfigFiles{}, err
	}

	err = writeMainConfig(data)
	if err != nil {
		return ConfigFiles{}, err
	}

	return data, nil
}

func writeMainConfig(c ConfigFiles) error {
	tmpl, err := template.New("").Parse(config)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.Config, buf.Bytes(), os.ModePerm)
}

func writeDynamicConfig(name string, ConfigFiles []proto.Message) error {
	resources := map[string][]json.RawMessage{}
	for _, config := range ConfigFiles {
		wrapped := anypb.Any{}
		err := anypb.MarshalFrom(&wrapped, config, proto.MarshalOptions{})
		if err != nil {
			return err
		}
		b, err := protojson.Marshal(&wrapped)
		if err != nil {
			return err
		}
		var object json.RawMessage
		json.Unmarshal(b, &object)
		resources["resources"] = append(resources["resources"], object)
	}
	b, err := json.Marshal(resources)
	if err != nil {
		return err
	}
	tmpName := name + ".tmp"
	err = os.WriteFile(tmpName, b, os.ModePerm)
	if err != nil {
		return err
	}
	// Envoy only responds to syscall.Rename.
	return os.Rename(tmpName, name)
}

func clustersToMessages(clusters []*clusterv3.Cluster) []proto.Message {
	messages := make([]proto.Message, 0, len(clusters))
	for _, cluster := range clusters {
		messages = append(messages, cluster)
	}
	return messages
}

func listenersToMessages(listeners []*listenerv3.Listener) []proto.Message {
	messages := make([]proto.Message, 0, len(listeners))
	for _, listener := range listeners {
		messages = append(messages, listener)
	}
	return messages
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
