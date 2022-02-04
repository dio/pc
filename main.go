package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/dio/pc/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	dir, err := ioutil.TempDir("", "pc")
	if err != nil {
		os.Exit(1)
	}

	funcEArgs := os.Args[2:]
	cfg := &config.Config{
		Path: funcEArgs[len(funcEArgs)-1],
		Dir:  dir,
		Args: funcEArgs,
	}
	name, err := cfg.Split()
	if err != nil {
		os.Exit(1)
	}
	fmt.Println(name)

	err = cfg.Watch(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
