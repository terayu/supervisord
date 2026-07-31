package main

import (
	"crypto/sha1" //nolint:gosec
	"crypto/subtle"
	"encoding/hex"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/rpc"
	"github.com/ochinchina/gorilla-xmlrpc/xml"
	"github.com/ochinchina/supervisord/process"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// XMLRPC mange the XML RPC servers
// start XML RPC servers to accept the XML RPC request from client side
type XMLRPC struct {
	// all the listeners to accept the XML RPC request
	listeners map[string]net.Listener
}

type httpBasicAuth struct {
	user     string
	password string
	handler  http.Handler
}

// create a new HttpBasicAuth object with username, password and the http request handler
func newHTTPBasicAuth(user string, password string, handler http.Handler) *httpBasicAuth {
	if user != "" && password != "" {
		log.Debug("require authentication")
	}
	return &httpBasicAuth{user: user, password: password, handler: handler}
}

func secureCompare(a string, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (h *httpBasicAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.user == "" || h.password == "" {
		log.Debug("no auth required")
		h.handler.ServeHTTP(w, r)
		return
	}
	username, password, ok := r.BasicAuth()
	if ok {
		// evaluate both the user name and the password unconditionally so that
		// the response time does not reveal which of them was wrong
		userOK := secureCompare(username, h.user)
		passOK := false
		if strings.HasPrefix(h.password, "{SHA}") {
			sum := sha1.Sum([]byte(password)) //nolint:gosec // kept for compatibility with supervisor
			passOK = secureCompare(hex.EncodeToString(sum[:]), h.password[5:])
		} else {
			passOK = secureCompare(password, h.password)
		}
		if userOK && passOK {
			h.handler.ServeHTTP(w, r)
			return
		}
	}
	if ok {
		// a request without any credential is the normal first half of the basic auth
		// handshake, only a wrong credential is worth warning about
		log.WithFields(log.Fields{"remote": r.RemoteAddr, "user": username}).Warn("authentication failed")
	}
	w.Header().Set("WWW-Authenticate", "Basic realm=\"supervisor\"")
	w.WriteHeader(401)
}

// NewXMLRPC create a new XML RPC object
func NewXMLRPC() *XMLRPC {
	return &XMLRPC{listeners: make(map[string]net.Listener)}
}

// Stop network listening
func (p *XMLRPC) Stop() {
	log.Info("stop listening")
	for _, listener := range p.listeners {
		listener.Close()
	}
	p.listeners = make(map[string]net.Listener)
}

// StartUnixHTTPServer start http server on unix domain socket with path listenAddr. If both user and password are not empty, the user
// must provide user and password for basic authentication when making an XML RPC request.
func (p *XMLRPC) StartUnixHTTPServer(user string, password string, listenAddr string, s *Supervisor, startedCb func()) {
	os.Remove(listenAddr)
	p.startHTTPServer(user, password, "unix", listenAddr, s, startedCb)
}

// StartInetHTTPServer start http server on tcp with path listenAddr. If both user and password are not empty, the user
// must provide user and password for basic authentication when making an XML RPC request.
//
// When no credential is configured, the server refuses to listen on a non-loopback address unless
// allowNoAuth is explicitly set in the configuration.
func (p *XMLRPC) StartInetHTTPServer(user string, password string, listenAddr string, allowNoAuth bool, s *Supervisor, startedCb func()) {
	if user == "" || password == "" {
		if isLoopbackListenAddr(listenAddr) {
			log.WithFields(log.Fields{"addr": listenAddr}).Warn("no username/password is configured, the HTTP interface is not protected")
		} else if allowNoAuth {
			log.WithFields(log.Fields{"addr": listenAddr}).Warn("no username/password is configured on a non-loopback address, but allow_no_auth is set; anyone who can reach this address can control supervisord")
		} else {
			log.WithFields(log.Fields{"addr": listenAddr}).Error("refuse to listen on a non-loopback address without username/password; set username/password, bind to 127.0.0.1, or set allow_no_auth=true")
			startedCb()
			return
		}
	}
	p.startHTTPServer(user, password, "tcp", listenAddr, s, startedCb)
}

func (p *XMLRPC) isHTTPServerStartedOnProtocol(protocol string) bool {
	_, ok := p.listeners[protocol]
	return ok
}

// isLoopbackListenAddr reports whether listenAddr binds to a loopback interface only.
// Unresolvable or non-IP hosts are treated as non-loopback (fail closed).
func isLoopbackListenAddr(listenAddr string) bool {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		// not a "host:port" form; cannot prove it is loopback
		return false
	}
	switch host {
	case "":
		// ":9001" means all interfaces
		return false
	case "*":
		return false
	case "localhost":
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// a hostname other than "localhost"; do not resolve at startup, fail closed
		return false
	}
	return ip.IsLoopback()
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func getProgramConfigPath(programName string, s *Supervisor) string {
	c := s.config.GetProgram(programName)
	if c == nil {
		return ""
	}

	res := c.GetString("conf_file", "")
	return res
}

// serveWebguiFile serves a file of the web GUI. It goes through HTTP instead of the
// local filesystem so that a release build sends its embedded copy; reading the file
// by relative path only worked when supervisord was started from the directory that
// contains webgui.
func serveWebguiFile(writer http.ResponseWriter, request *http.Request, name string) {
	f, err := HTTP.Open(name)
	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	http.ServeContent(writer, request, info.Name(), info.ModTime(), f)
}

func readLogHtml(writer http.ResponseWriter, request *http.Request) {
	serveWebguiFile(writer, request, "log.html")
}

func (p *XMLRPC) startHTTPServer(user string, password string, protocol string, listenAddr string, s *Supervisor, startedCb func()) {
	if p.isHTTPServerStartedOnProtocol(protocol) {
		startedCb()
		return
	}
	procCollector := process.NewProcCollector(s.procMgr)
	prometheus.Register(procCollector)
	mux := http.NewServeMux()
	mux.Handle("/RPC2", newHTTPBasicAuth(user, password, p.createRPCServer(s)))

	progRestHandler := NewSupervisorRestful(s).CreateProgramHandler()
	mux.Handle("/program/", newHTTPBasicAuth(user, password, progRestHandler))

	supervisorRestHandler := NewSupervisorRestful(s).CreateSupervisorHandler()
	mux.Handle("/supervisor/", newHTTPBasicAuth(user, password, supervisorRestHandler))

	// deprecated, it is buggy
	logtailHandler := NewLogtail(s).CreateHandler()
	mux.Handle("/logtail/", newHTTPBasicAuth(user, password, logtailHandler))

	webguiHandler := NewSupervisorWebgui(s).CreateHandler()
	mux.Handle("/", newHTTPBasicAuth(user, password, webguiHandler))

	// conf file
	confHandler := NewConfApi(s).CreateHandler()
	mux.Handle("/conf/", newHTTPBasicAuth(user, password, confHandler))
	mux.Handle("/confFile", newHTTPBasicAuth(user, password, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		serveWebguiFile(writer, request, "conf.html")
	})))

	// read the log.html file
	mux.Handle("/log", newHTTPBasicAuth(user, password, http.HandlerFunc(readLogHtml)))

	mux.Handle("/metrics", newHTTPBasicAuth(user, password, promhttp.Handler()))

	// register the log routes so that the log directory can be browsed
	entryList := s.config.GetPrograms()
	for _, c := range entryList {
		realName := c.GetProgramName()
		if realName == "" {
			continue
		}

		filePath := c.GetString("stdout_logfile", "")
		if filePath == "" {
			continue
		}
		dir := filepath.Dir(filePath)
		prefix := "/log/" + realName + "/"
		mux.Handle(prefix, newHTTPBasicAuth(user, password,
			http.StripPrefix(prefix, http.FileServer(http.Dir(dir)))))
	}

	listener, err := net.Listen(protocol, listenAddr)
	if err == nil {
		if protocol == "unix" {
			// restrict the socket to its owner; without this any local user can control supervisord
			if chmodErr := os.Chmod(listenAddr, 0700); chmodErr != nil {
				listener.Close()
				startedCb()
				log.WithFields(log.Fields{"addr": listenAddr, "error": chmodErr}).Error("fail to restrict permission of the unix socket, refuse to serve on it")
				return
			}
		}
		log.WithFields(log.Fields{"addr": listenAddr, "protocol": protocol}).Info("success to listen on address")
		p.listeners[protocol] = listener
		startedCb()
		http.Serve(listener, mux)
	} else {
		startedCb()
		log.WithFields(log.Fields{"addr": listenAddr, "protocol": protocol}).Fatal("fail to listen on address")
	}

}
func (p *XMLRPC) createRPCServer(s *Supervisor) *rpc.Server {
	RPC := rpc.NewServer()
	xmlrpcCodec := xml.NewCodec()
	RPC.RegisterCodec(xmlrpcCodec, "text/xml")
	RPC.RegisterService(s, "")

	xmlrpcCodec.RegisterAlias("supervisor.getVersion", "Supervisor.GetVersion")
	xmlrpcCodec.RegisterAlias("supervisor.getAPIVersion", "Supervisor.GetVersion")
	xmlrpcCodec.RegisterAlias("supervisor.getIdentification", "Supervisor.GetIdentification")
	xmlrpcCodec.RegisterAlias("supervisor.getState", "Supervisor.GetState")
	xmlrpcCodec.RegisterAlias("supervisor.getPID", "Supervisor.GetPID")
	xmlrpcCodec.RegisterAlias("supervisor.readLog", "Supervisor.ReadLog")
	xmlrpcCodec.RegisterAlias("supervisor.clearLog", "Supervisor.ClearLog")
	xmlrpcCodec.RegisterAlias("supervisor.shutdown", "Supervisor.Shutdown")
	xmlrpcCodec.RegisterAlias("supervisor.restart", "Supervisor.Restart")
	xmlrpcCodec.RegisterAlias("supervisor.getProcessInfo", "Supervisor.GetProcessInfo")
	xmlrpcCodec.RegisterAlias("supervisor.getSupervisorVersion", "Supervisor.GetVersion")
	xmlrpcCodec.RegisterAlias("supervisor.getAllProcessInfo", "Supervisor.GetAllProcessInfo")
	xmlrpcCodec.RegisterAlias("supervisor.startProcess", "Supervisor.StartProcess")
	xmlrpcCodec.RegisterAlias("supervisor.startAllProcesses", "Supervisor.StartAllProcesses")
	xmlrpcCodec.RegisterAlias("supervisor.startProcessGroup", "Supervisor.StartProcessGroup")
	xmlrpcCodec.RegisterAlias("supervisor.stopProcess", "Supervisor.StopProcess")
	xmlrpcCodec.RegisterAlias("supervisor.stopProcessGroup", "Supervisor.StopProcessGroup")
	xmlrpcCodec.RegisterAlias("supervisor.stopAllProcesses", "Supervisor.StopAllProcesses")
	xmlrpcCodec.RegisterAlias("supervisor.signalProcess", "Supervisor.SignalProcess")
	xmlrpcCodec.RegisterAlias("supervisor.signalProcessGroup", "Supervisor.SignalProcessGroup")
	xmlrpcCodec.RegisterAlias("supervisor.signalAllProcesses", "Supervisor.SignalAllProcesses")
	xmlrpcCodec.RegisterAlias("supervisor.sendProcessStdin", "Supervisor.SendProcessStdin")
	xmlrpcCodec.RegisterAlias("supervisor.sendRemoteCommEvent", "Supervisor.SendRemoteCommEvent")
	xmlrpcCodec.RegisterAlias("supervisor.reloadConfig", "Supervisor.ReloadConfig")
	xmlrpcCodec.RegisterAlias("supervisor.addProcessGroup", "Supervisor.AddProcessGroup")
	xmlrpcCodec.RegisterAlias("supervisor.removeProcessGroup", "Supervisor.RemoveProcessGroup")
	xmlrpcCodec.RegisterAlias("supervisor.readProcessStdoutLog", "Supervisor.ReadProcessStdoutLog")
	xmlrpcCodec.RegisterAlias("supervisor.readProcessStderrLog", "Supervisor.ReadProcessStderrLog")
	xmlrpcCodec.RegisterAlias("supervisor.tailProcessStdoutLog", "Supervisor.TailProcessStdoutLog")
	xmlrpcCodec.RegisterAlias("supervisor.tailProcessStderrLog", "Supervisor.TailProcessStderrLog")
	xmlrpcCodec.RegisterAlias("supervisor.clearProcessLogs", "Supervisor.ClearProcessLogs")
	xmlrpcCodec.RegisterAlias("supervisor.clearAllProcessLogs", "Supervisor.ClearAllProcessLogs")
	return RPC
}
