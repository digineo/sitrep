module github.com/digineo/sitrep

go 1.27.1

ignore ./frontend/node_modules

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/digineo/xlog v1.1.0
	github.com/digineo/xlog/slogor v1.1.0
	github.com/stretchr/testify v1.12.1
	github.com/yuin/goldmark v1.8.6
	go.etcd.io/bbolt v1.5.0
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/crypto v0.57.0
	golang.org/x/oauth2 v0.37.0
	golang.org/x/term v0.46.0
)

require (
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	gitlab.com/greyxor/slogor v1.7.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
