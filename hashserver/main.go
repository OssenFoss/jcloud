// Package main: provides an http server that provides a base64 encoded string of teh SHA512 hash of the provided password
package main

import (
    "github.com/ossenfoss/jcloud/hashhttpserver"
    "fmt"
    "os"
    "path"
    "strconv"
)

func Usage() {
    fmt.Println("go run", path.Base(os.Args[0]), "PortNumber")
    os.Exit(-1)
}
func main() {

    var portnum int
    var err error
    if len(os.Args) > 1 {
	// Get port number
        if portnum, err = strconv.Atoi(os.Args[1]); err != nil || portnum == 0 {
	    fmt.Println(err)
	    Usage()
	}
    } else {
	Usage()
    }

    hashhttpserver.ListenAndServe(portnum)
}
