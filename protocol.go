package routerosv7

import (
	"log"
	"net"
)

// isHostAvailableOnPort checks if a host is available on a given port.
func isHostAvailableOnPort(host, port string) bool {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return false
	}
	err = conn.Close()
	if err != nil {
		log.Println(err)
	}
	return true
}
