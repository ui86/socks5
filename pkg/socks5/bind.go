package socks5

import (
	"errors"
	"net"
)

// TODO
func (r *Request) bind(_ net.Conn) error {
	return errors.New("Unsupport BIND now")
}
