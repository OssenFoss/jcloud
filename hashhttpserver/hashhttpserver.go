// Package hashhttpserver: provides an http server that provides a base64 encoded string of teh SHA512 hash of the provided password
package hashhttpserver

import (
    b64 "encoding/base64"
    "context"
    "fmt"
    "encoding/json"
    "net/http"
    "log"
    "crypto/sha512"
    "strconv"
    "sync"
    "time"
)

// Creates an http server w/ async request handling.
// Uses net/http 

// if shutting down - reject new hash requests
// The handler dispatches to hash/stats/shutdown
// All other requests consult a map on in progress hash requests.
// If the map location is empty - return error (404 - either results already returned or no request made)
// If map holds results - return the results and clear the map
// If map holds an incomplete context - work is still in progress, return 200 
// The map is a contended structure and needs synchronzation
//
// shutdown - wait for map to be empty (could be forever - need to clarify requirements
// then cleanly shutdown the http server
//
// Stats requirement is a bit unclear - says to measure average post request time.
// I am assuming this means the actual post request handling, not the 5 second delay
//
// stats are contended structures and need syncrhonization


type ResultsMap struct {
    sync.Mutex				// lock across counter and map
    respIdCounter 	int64		// monotonically increasing response id 
    asyncResults map[string]string	// String will be "" if no result available yet
}

//================ Stats ===============
type Stats struct {
    sync.Mutex
    count	int64 // How many POSTs recorded
    totalTime  time.Duration // accumulated runtime
}

// addMeasurement: records another measurement and its duration
func (stat *Stats) addMeasurement(duration time.Duration) {
    stat.Lock()
    defer stat.Unlock()
    // Record another measurement
    stat.count += 1
    // Add duration to total time
    stat.totalTime += duration
}

// getStats: gets the current number of measurements and the average time of them
func (stat *Stats) getStats() map[string]int64 {
    stat.Lock()
    defer stat.Unlock()
    retstats := make(map[string]int64)
    retstats["total"] = stat.count
    if stat.count != 0 {
        retstats["average"] = int64(stat.totalTime.Microseconds()/stat.count)
    } else { 
        retstats["average"] = 0
    }
    return retstats
}
//^================ Stats ===============

//================ HashHttpServer ============
type HashHttpServer struct {
    htserver	*http.Server
    handler     *http.ServeMux
    shutdown	bool
    waitg	sync.WaitGroup 
    asyncReq	ResultsMap
    stats	Stats
}

// Construct the HashHttpServer - ctor need to properly init the map
func NewHashHttpServer(portnum int) *HashHttpServer {
    var server HashHttpServer
    server.asyncReq.asyncResults = make(map[string]string)
    server.shutdown = false
    server.handler = http.NewServeMux()
    server.htserver = &http.Server {
	Addr:           ":"+strconv.FormatInt(int64(portnum), 10),
	Handler:        server.handler,
	}

    return &server
}

func (server *HashHttpServer) HandleFunc(pattern string,
					 handler func(*HashHttpServer, http.ResponseWriter, *http.Request)) {
    lamfunc := func(resp http.ResponseWriter, req *http.Request) {
                        handler(server, resp, req)
    }
    server.handler.HandleFunc(pattern, lamfunc)
 
}

// RegisterAsyncReturn: registers for a future return value.
// returns a string responseId that can be used to update or fetch the eventual return value
func (server *HashHttpServer) RegisterAsyncReturn(notdonevalue string) (responseId string) {
    // Lock the request map and respIdCounter
    server.asyncReq.Lock()
    defer server.asyncReq.Unlock()

    // Get our response id
    server.asyncReq.respIdCounter += 1
    responseId = strconv.FormatInt(server.asyncReq.respIdCounter, 10)
    log.Println("INFO: ResponseId", responseId, server.stats.count)

    // Mark response as not (yet) available
    server.asyncReq.asyncResults[responseId] = ""

    return
}

// SetAsyncReturnValue: records the return value for the given responseId to be picked up by a later request
func (server *HashHttpServer) SetAsyncReturnValue(responseId string, returnValue string) {
    server.asyncReq.Lock()
    defer server.asyncReq.Unlock()
    server.asyncReq.asyncResults[responseId] = returnValue
}

/* GetAsyncReturnValue: Gets the return value for the given responseId.
If the responseId is not registered (either never used or already returned, return ok < 0
If the registered value is == notdonevalue, return ok > 0
If the registered value is != notdonevalue, caller is picking up the results, ok = 0
if ok == 0, the responseId is unregistered and the return value is removed.
*/
func (server *HashHttpServer) GetAsyncReturnValue(responseId string, notdonevalue string) (results string, ok int) {
    // Lock the request map
    server.asyncReq.Lock()
    defer server.asyncReq.Unlock()

    var found bool
    if results, found = server.asyncReq.asyncResults[responseId]; found {
        if results == notdonevalue {
            // Results not available yet
            // Return accepted
            log.Println("INFO: Request for", responseId, "not available (yet)")
	    ok = 1
        } else {
            // Results are availabe - return then and remove them from map
            log.Println("INFO: Request for", responseId, results)
            delete(server.asyncReq.asyncResults, responseId)
	    ok = 0
	}
     } else {
	// Invalid path/responseId - we have never heard of it (or results were already returned)
	log.Println("INFO: Request for", responseId, "unknown")
	ok = -1
     }
     return
}

//^================ HashHttpServer ============

// hash: handler for post requests for hash
//  Spawns and thread to perform the actual hashing (since it takes too much time)
func hash(server *HashHttpServer, resp http.ResponseWriter, req *http.Request) {
    // Start recording duration 
    start := time.Now()
    defer func() {
            // Add duration to total time
            duration := time.Since(start)
	    server.stats.addMeasurement(duration)
	}()

    if server.shutdown {
	// Server is shutting down - reject new requests
	resp.WriteHeader(503) // Return 503 Service Unavailable
	return
    }
    if req.Method != "POST" {
	// ensure post only 
	resp.WriteHeader(405) // Return 405 Method Not Allowed.
	return
    }

    // Parse the raw query and update req.Form.
    if err := req.ParseForm(); err != nil {
        log.Println("ERROR: ParseForm() err: %v\n", err)
        fmt.Fprintf(resp, "ParseForm() err: %v", err)
        return
    }

    password := req.PostFormValue("password")
    //Don't log passwords fmt.Println("Password is", password)
    if password == "" {
        resp.WriteHeader(422) // Unprocessable Entity
        return
    }

    // Mark our spot for our future return value
    responseId := server.RegisterAsyncReturn("")

    // return async request number
    fmt.Fprintf(resp, "%s\n", responseId)

    // Start a go func to calculate hash in background
    // record us started for clean shutdowns
    server.waitg.Add(1)
    // Spinup async request
    go asyncHashHandler(password, server, responseId)

}

// asyncHashHandler: the function that actually does the hashing and posts the results
// into the map (to be picked up later by asyncReturns)
func asyncHashHandler(password string, server *HashHttpServer, responseId string) {
    log.Println("INFO: asyncHashHandler started")
    time.Sleep(5 * time.Second)


    // Get sha512 of the password
    hashedPass := sha512.Sum512([]byte(password))
    encodedHashedPass := b64.StdEncoding.EncodeToString(hashedPass[:])
    //fmt.Println("sha 512", len(hashedPass), hashedPass)
    //fmt.Println("encodedHashedPass", encodedHashedPass)
    
    server.SetAsyncReturnValue(responseId, encodedHashedPass)
    
    // record us done for clean shutdowns
    server.waitg.Done()
    log.Println("INFO: asyncHashHandler finished")
}

// asyncReturns: handler for any unknown url AND for requests for async returns.
// If the path isn't in the map - givem the old 404
// If in the map, it means the hash isn't calculated yet (202)
func asyncReturns(server *HashHttpServer, resp http.ResponseWriter, req *http.Request) {
    if req.Method != "GET" {
	// ensure GET only 
	resp.WriteHeader(405) // Return 405 Method Not Allowed.
	return
    }
    // Extract the responseId from the URL
    responseId := req.URL.Path[1:] // Slice off leading /


    results, ok := server.GetAsyncReturnValue(responseId, "")
    log.Println("GetAsyncReturnValue returned'", results, "' ok=", ok)
   
    if ok > 0 {
	// Not done
	resp.WriteHeader(202)
    } else if ok == 0 {
        // Results done and available
	fmt.Fprintf(resp, results)
    } else { 
        // ResponseId not registered
	resp.WriteHeader(404)
    }
}

// stats: handler for requests for stats.
func stats(server *HashHttpServer, resp http.ResponseWriter, req *http.Request) {
    if req.Method != "GET" {
	// ensure GET only 
	resp.WriteHeader(405) // Return 405 Method Not Allowed.
	return
    }
    //fmt.Fprintf(resp, "stats")
    // get the stats
    retstats := server.stats.getStats()
    retjs, err := json.Marshal(retstats)
    if err != nil {
      http.Error(resp, err.Error(), http.StatusInternalServerError)
      return
    }

    resp.Write(retjs)
}

// shutdown: handles requests to shutdown the server. net/http/Server provides a graceful 
// shutdown but, we must also wait for any background calcs (hash) going on. Use wait group 
// to handle waiting for those.
func shutdown(server *HashHttpServer, resp http.ResponseWriter, req *http.Request) {
    if req.Method != "GET" {
	// ensure GET only 
	resp.WriteHeader(405) // Return 405 Method Not Allowed.
	return
    }
    // Stop new requests from coming in
    server.shutdown = true

    // Wait for threads and server to shutdown in another thread so we can close this connection
    go func() {
        // Wait for in progress request routines to finish
        server.waitg.Wait()

	if err := server.htserver.Shutdown(context.Background()); err != nil {
		// Error from closing listeners, or context timeout:
		log.Fatal(err)
		}

	}()

    fmt.Fprintf(resp, "shutting down")
}


// ListenAndServe: the primary entry point. This configures and starts the http server, then
// waits for the server to be shutdown.
func ListenAndServe(portnum int) {

    server := NewHashHttpServer(portnum)

    server.HandleFunc("/stats", stats)
    server.HandleFunc("/shutdown", shutdown)
    server.HandleFunc("/hash", hash)
    server.HandleFunc("/", asyncReturns)

    log.Fatal(server.htserver.ListenAndServe())
}
