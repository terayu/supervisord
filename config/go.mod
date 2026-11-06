module github.com/terayu/supervisord/config

go 1.24.0

toolchain go1.24.4

require (
	github.com/hashicorp/go-envparse v0.1.0
	github.com/ochinchina/go-ini v1.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/terayu/supervisord/util v0.0.0-20250610055946-d5a5470d11af
)

require golang.org/x/sys v0.33.0 // indirect

replace github.com/terayu/supervisord/util => ../util
