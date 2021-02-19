
Not so much tests, as excercisers.
Mostly requires visual inspection of results.

post PORT ID
Sends a hash request to the server (on localhost and always the same password).
Followed by a request to get the async return (which should always be not ready)
PORT == portnumber, ID is expected identifier for the async return

response PORT ID
Sends a request for the specified async return

responsehit PORT ID
Spins in a loop checking the async return of the specified ID - spins until status changes from 404
Then spins on 202 code waiting till the hash is calculated, gets and displays it.
Then checks the ID one more time to be sure we get 404 again

stats PORT
Sends a requenst for stats

shutdown PORT 
Sends a shutdown request

badpost - sends in ill-formated password
*Post - just like above but send the wrong method
