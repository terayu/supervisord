module github.com/terayu/supervisord/process

go 1.24.0

require (
	github.com/mitchellh/go-ps v1.0.0
	github.com/ochinchina/filechangemonitor v0.3.1
	github.com/prometheus/client_golang v1.22.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/terayu/supervisord/config v0.0.0-20250610055946-d5a5470d11af
	github.com/terayu/supervisord/events v0.0.0-20250610055946-d5a5470d11af
	github.com/terayu/supervisord/logger v0.0.0-20250610055946-d5a5470d11af
	github.com/terayu/supervisord/signals v0.0.0-20250610055946-d5a5470d11af
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gorilla/rpc v1.2.1 // indirect
	github.com/hashicorp/go-envparse v0.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ochinchina/go-ini v1.0.1 // indirect
	github.com/ochinchina/gorilla-xmlrpc v0.0.0-20171012055324-ecf2fe693a2c // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.64.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rogpeppe/go-charset v0.0.0-20190617161244-0dc95cdf6f31 // indirect
	github.com/terayu/supervisord/faults v0.0.0-20250610055946-d5a5470d11af // indirect
	github.com/terayu/supervisord/util v0.0.0-20250610055946-d5a5470d11af // indirect
	golang.org/x/sys v0.33.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/terayu/supervisord/config => ../config

replace github.com/terayu/supervisord/events => ../events

replace github.com/terayu/supervisord/logger => ../logger

replace github.com/terayu/supervisord/signals => ../signals

replace github.com/terayu/supervisord/faults => ../faults

replace github.com/terayu/supervisord/util => ../util

replace github.com/terayu/supervisord/types => ../types

replace github.com/terayu/supervisord/xmlrpcclient => ../xmlrpcclient
