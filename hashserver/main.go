// Package main: provides an http server that provides a base64 encoded string of teh SHA512 hash of the provided password
package main

import (
    "github.com/ossenfoss/jcloud/hashhttpserver"
    "flag"
    "fmt"
    "log"
    "os"
)

func main() {
    var portnum int
    var logfile string
    flag.StringVar(&logfile, "logfile", "", "file name for logging information (if not specified, logging is to stderr)")
    flag.IntVar(&portnum, "port", 8080, "the port the server will listen on")
    flag.Usage = func() {
	    fmt.Println("Usage: hashserver")
	    flag.PrintDefaults()
	}
    flag.Parse()

    fmt.Println("Parms", portnum, logfile)
    // If the file doesn't exist, create it or append to the file
    if logfile != "" {
        file, err := os.OpenFile(logfile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
    	    if err != nil {
                log.Fatal(err)
            }
        log.SetOutput(file)
    }
    log.Println("Starting hashserver")
    hashhttpserver.ListenAndServe(portnum)
}
