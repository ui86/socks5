package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"socks5/pkg/socks5"
	"strconv"
	"strings"
)

var (
	port      int
	username  string
	password  string
	whiteList string
)

func init() {
	flag.StringVar(&username, "user", "", "username")
	flag.StringVar(&password, "pwd", "", "password")
	flag.IntVar(&port, "p", 1080, "port on listen, must be greater than 0")
	flag.StringVar(&whiteList, "whitelist", "", "comma-separated list of allowed IP addresses (e.g. '127.0.0.1,1.1.1.1')")
	flag.Parse()
}

func main() {
	log.Println("Welcome use socks5 server")
	if port <= 0 {
		flag.Usage()
		os.Exit(1)
	}

	var serverAddr *net.TCPAddr
	if addr, err := net.ResolveTCPAddr("tcp", ":"+strconv.Itoa(port)); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	} else {
		serverAddr = addr
	}

	// Parse whitelist
	var whitelistIPs []string
	if whiteList != "" {
		whitelistIPs = strings.Split(whiteList, ",")
		for i := range whitelistIPs {
			whitelistIPs[i] = strings.TrimSpace(whitelistIPs[i])
		}
	}

	if len(whitelistIPs) == 0 {
		log.Println("Warning: whitelist is empty, all IPs are allowed")
	} else {
		log.Printf("Whitelist: %v\n", whitelistIPs)
	}

	s, err := socks5.NewClassicServer(serverAddr.String(), "0.0.0.0", username, password, 0, 60, whitelistIPs)
	if err != nil {
		log.Println(err)
		return
	}

	log.Printf("Server is listening on %s\n", serverAddr.String())

	// Start server
	if err := s.ListenAndServe(nil); err != nil {
		log.Println(err)
		return
	}

}
