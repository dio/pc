package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/dio/pc/internal/archives"
	"github.com/dio/pc/internal/downloader"
	"github.com/dio/pc/internal/handler"
	"github.com/dio/pc/internal/runner"
	"github.com/mitchellh/go-homedir"
	"github.com/oklog/run"
)

const (
	funcEHomeEnvKey = "FUNC_E_HOME"
)

// go run github.com/dio/pc@main func-e run ...
// go run github.com/dio/pc@main -c ... (pass args to envoy)
func main() {
	// The func-e home is defined by $FUNC_E_HOME or if not defined: ~/.func-e.
	// The binary should be downloaded to $FUNC_E_HOME/versions/1.20.1/bin/envoy
	funcEHome := os.Getenv(funcEHomeEnvKey)
	if funcEHome == "" {
		home, _ := homedir.Dir()
		funcEHome = filepath.Join(home, ".func-e")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Use whatever func-e has downloaded, or download the default one.
	archive := &archives.Proxy{VersionUsed: proxyVersion(funcEHome)}
	binaryPath, err := downloader.DownloadVersionedBinary(ctx, archive, funcEHome)
	if err != nil {
		fmt.Printf("failed to download proxy binary: %v\n", err)
		os.Exit(1)
	}

	watcher, cancel := handler.New(ctx, os.Args[1:])
	fmt.Printf("admin address: %s\n", watcher.Files().AdminAddressPath)

	var g run.Group
	{
		g.Add(watcher.Run, cancel)
	}
	{
		binary, cancel := runner.New(ctx, binaryPath, watcher.Args(), nil)
		g.Add(binary.Run, cancel)
	}
	err = g.Run()
	if err != nil {
		fmt.Printf("failed to run: %v\n", err)
		os.Exit(1)
	}
}

func proxyVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "version"))
	if err != nil {
		return ""
	}
	captured := string(b)
	parts := strings.Split(captured, ".")
	if len(parts) == 3 {
		return captured
	}
	for i := 0; i < 20; i++ {
		version := fmt.Sprintf(captured+".%d", i)
		_, err = os.Lstat(filepath.Join(dir, "versions", version))
		if err == nil {
			return version
		}
	}
	return ""
}
