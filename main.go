package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"socks5/pkg/socks5"
	"strconv"
)

var (
	port     int
	username string
	password string
)

func init() {
	flag.StringVar(&username, "user", "", "username")
	flag.StringVar(&password, "pwd", "", "password")
	flag.IntVar(&port, "p", 1080, "port on listen, must be greater than 0")
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

	s, err := socks5.NewClassicServer(serverAddr.String(), "0.0.0.0", username, password, 0, 60)
	if err != nil {
		log.Println(err)
		return
	}
	if err := s.ListenAndServe(nil); err != nil {
		log.Println(err)
		return
	}

}
