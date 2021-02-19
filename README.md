# jcloud
Code for our good friends ...

# To start the server
    go run hashserver/main.go -port 8080 -logfile logs    

#
    Usage: hashserver
      -logfile string
    	    file name for logging information (if not specified, logging is to stderr)
      -port int
    	    the port the server will listen on (default 8080)

See tests directory for some excercisers.

Examples:
    > curl --data 'password=happykitten' http://localhost:8080/hash
    1

    > curl http://localhost:8080/1
    vcrchOTM2yko83+k7fHgGojyt80U6TpcBFqYufklDcres+mxfbTAmCQqdQR4m/opGHyL3OZkRy0RT1CRNjhRyQ==%  

    > curl http://localhost:8080/stats
    {"average":27,"total":5}%   

    curl http://localhost:8080/shutdown
    shutting down%            
