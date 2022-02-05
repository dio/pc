To create the archive, do the following in this directory:

```console
tar cfz archive.tar.gz auth_server.stripped
```

This is exactly the same as the archive structure for auth_server. You can check it with:

```console
curl -sSL https://github.com/dio/authservice/releases/download/v0.6.0-rc0/auth_server_0.6.0-rc0_darwin_amd64.tar.gz | tar tvz -
```
