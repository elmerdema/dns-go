package main

import (
	"fmt"
	"net"
	"strings"
)

type Serve interface {
	ServeDNS(writer net.Conn, message string)
}

type DNSHandler struct {
}

func (d DNSHandler) ServeDNS(writer net.Conn, message string) {
	question := strings.TrimSpace(message)

	ip, found := sites[question]
	if found {
		fmt.Fprintf(writer, "The ip for %s has been found ", ip)
	} else {
		fmt.Printf("No ip found for %s in %s \n", ip, question)
	}
}

func Lookup(s Serve, conn net.Conn, message string) {
	s.ServeDNS(conn, message)

}

func main() {
	initSites()
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	buffer := make([]byte, 1024)
	n, _, _ := conn.ReadFromUDP(buffer)
	handler := DNSHandler{}
	handler.ServeDNS(conn, string(buffer[:n]))
}
