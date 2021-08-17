package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"runtime"

	"github.com/semrush/zenrpc"
)

/* global variable declaration, if any... */

// const no = "no"
// const on = "on"

const off = "off"
const undefined = "undefined"
const unknown = "unknown"
const yes = "yes"
const app = "libvirt-jrpc"

// JRPCService - zenrpc.Service struct
type JRPCService struct{ zenrpc.Service }

var (
	info *log.Logger
	fail *log.Logger

	buildDate = undefined
	gitBranch = undefined
	gitState  = undefined
	gitCommit = undefined

	ip     *string
	port   *int
	socket *string
)

func init() {

	if runtime.NumCPU() > 1 {
		runtime.GOMAXPROCS(2)
	}

	userDef, err := user.Current()
	if err != nil {
		log.Fatalf("Failed to get current user info: %s", err.Error())
	}

	if userDef.Uid != "0" {
		log.Fatalf("@_@ Program should be run with root privileges!")
	}

	logToFileDesc := fmt.Sprintf("write events to main log \"/var/log/%s-main.log\" and errors log to \"/var/log/%s-errors.log\"", app, app)
	logToFilePtr := flag.Bool("log-to-files", false, logToFileDesc)

	ip = flag.String("ip", "127.0.0.1", "IP that JRPC server will bind to")
	port = flag.Int("port", 8888, "port number that JRPC server will bind to")
	socket = flag.String("unix-socket", "", "path to Unix domain socket insted of IP that JRPC server will bind to")

	flag.Parse()

	if *logToFilePtr {
		mainLog, err := os.OpenFile(fmt.Sprintf("/var/log/%s-main.log", app), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Failed to open log file: %s", err.Error())
		}

		errorsLog, err := os.OpenFile(fmt.Sprintf("/var/log/%s-errors.log", app), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Failed to open log file: %s", err.Error())
		}

		info = log.New(io.MultiWriter(os.Stdout, mainLog), "INF: ", log.LstdFlags|log.Lshortfile)
		fail = log.New(io.MultiWriter(os.Stdout, mainLog, errorsLog), "ERR: ", log.LstdFlags|log.Lshortfile)
	} else {
		info = log.New(os.Stdout, "INF: ", log.LstdFlags|log.Lshortfile)
		fail = log.New(os.Stdout, "ERR: ", log.LstdFlags|log.Lshortfile)
	}

}

func main() {

	info.Printf("Build Date: %s, Git Branch: %s, Git State: %s, Git Commit: %s", buildDate, gitBranch, gitState, gitCommit)

	jrpc := zenrpc.NewServer(zenrpc.Options{
		BatchMaxLen:            1,
		TargetURL:              "jrpc",
		ExposeSMD:              false,
		DisableTransportChecks: false,
		AllowCORS:              true,
	})

	jrpc.Register("jrpc", JRPCService{})
	jrpc.Register("", JRPCService{}) // public
	jrpc.Use(logger())

	if len(*socket) == 0 {

		mux := http.NewServeMux()
		mux.Handle("/jrpc", jrpc)
		mux.Handle("/", loggingHandler(http.FileServer(http.Dir("ui"))))

		info.Printf("Starting JRPC server on %s:%d", *ip, *port)

		err := http.ListenAndServe(fmt.Sprintf("%s:%d", *ip, *port), limitHandler(mux))
		if err != nil {
			fail.Fatalf("JRPC server crashed: %s", err.Error())
		}

	} else {

		http.Handle("/jrpc", jrpc)
		http.Handle("/", loggingHandler(http.FileServer(http.Dir("ui"))))

		info.Printf("Starting JRPC server on %s", *socket)

		uL, err := net.Listen("unix", *socket)
		if err != nil {
			fail.Fatalf("JRPC server crashed: %s", err.Error())
		}

		if err := os.Chmod(*socket, 0777); err != nil {
			fail.Fatalf("JRPC server crashed: %s", err.Error())
		}

		err = http.Serve(uL, nil)
		if err != nil {
			fail.Fatalf("JRPC server crashed: %s", err.Error())
		}

	}
}
