# pc

`pc` is a proxy config tool. This is a [`func-e`](https://func-e.io/) companion. While you can run
`pc` without `func-e`, it is recommended to check that fantastic tool, especially when you want
to manage multiple versions of proxies.

## Install

```console
go install github.com/dio/pc@main
```

## To run

If you have [Go 1.17.x](https://go.dev/doc/install) installed on your system:

```console
go run github.com/dio/pc@main -c internal/handler/testdata/config.yaml
```

When you have this repo downloaded:

```console
go run main.go -c internal/handler/testdata/config.yaml
```

While the following also works:

```console
go run main.go func-e run -c internal/handler/testdata/config.yaml
```
