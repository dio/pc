package config

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"

	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3" // to resolve missing type URL
	_ "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"                          // to resolve missing type URL
	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"sigs.k8s.io/yaml"
)

//go:embed template/config.yaml
var config string

type Config struct {
	Path string
	Dir  string
	Args []string
	disc Discovery
}

func (c *Config) Watch(ctx context.Context) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					_, err := c.Split()
					if err != nil {
						return
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			case <-cancelCtx.Done():
				return
			}
		}
	}()

	watcher.Add(c.Path)
	if err != nil {
		return err
	}

	c.Args[len(c.Args)-1] = c.disc.Main
	cmd := exec.CommandContext(cancelCtx, os.Args[1], c.Args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		fmt.Println(err)
		cancel()
		os.Exit(1)
	}

	<-cancelCtx.Done()

	if runtime.GOOS == "darwin" {
		syscall.Kill(cmd.Process.Pid+1, syscall.SIGINT)
		syscall.Kill(cmd.Process.Pid+1, syscall.SIGKILL)
	}
	return nil
}

type Discovery struct {
	Clusters  string
	Listeners string
	Main      string
}

func (c *Config) Split() (string, error) {
	c.disc = Discovery{
		Clusters:  filepath.Join(c.Dir, "clusters.json"),
		Listeners: filepath.Join(c.Dir, "listeners.json"),
		Main:      filepath.Join(c.Dir, "config.yaml"),
	}

	content, err := os.ReadFile(c.Path)
	if err != nil {
		return "", err
	}

	j, err := yaml.YAMLToJSON(content)
	if err != nil {
		return "", err
	}

	var bootstrap bootstrapv3.Bootstrap
	if err := protojson.Unmarshal(j, &bootstrap); err != nil {
		return "", err
	}

	resources := map[string][]json.RawMessage{}
	for _, cluster := range bootstrap.StaticResources.Clusters {
		clusterAny := anypb.Any{}
		if err = anypb.MarshalFrom(&clusterAny, cluster, proto.MarshalOptions{}); err != nil {
			return "", err
		}

		b, err := protojson.Marshal(&clusterAny)
		if err != nil {
			return "", err
		}
		var clusterObject json.RawMessage
		json.Unmarshal(b, &clusterObject)
		resources["resources"] = append(resources["resources"], clusterObject)
	}

	b, err := json.Marshal(resources)
	if err != nil {
		return "", err
	}

	if err = os.WriteFile(c.disc.Clusters+".tmp", b, os.ModePerm); err != nil {
		return "", err
	}

	if err = os.Rename(c.disc.Clusters+".tmp", c.disc.Clusters); err != nil {
		return "", err
	}

	resources = map[string][]json.RawMessage{}
	for _, listener := range bootstrap.StaticResources.Listeners {
		listenerAny := anypb.Any{}
		if err = anypb.MarshalFrom(&listenerAny, listener, proto.MarshalOptions{}); err != nil {
			return "", err
		}

		b, err := protojson.Marshal(&listenerAny)
		if err != nil {
			return "", err
		}
		var listenerObject json.RawMessage
		json.Unmarshal(b, &listenerObject)
		resources["resources"] = append(resources["resources"], listenerObject)
	}

	b, err = json.Marshal(resources)
	if err != nil {
		return "", err
	}

	if err = os.WriteFile(c.disc.Listeners+".tmp", b, os.ModePerm); err != nil {
		return "", err
	}

	if err = os.Rename(c.disc.Listeners+".tmp", c.disc.Listeners); err != nil {
		return "", err
	}

	tmpl, err := template.New("").Parse(config)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	tmpl.Execute(buf, c.disc)

	if err = os.WriteFile(c.disc.Main, buf.Bytes(), os.ModePerm); err != nil {
		return "", err
	}
	return c.disc.Main, err
}
