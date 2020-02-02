We can add a Load balancing layer at three places in our system:
1. Between client and application servers
2. Between application server and database
3. Between application server and cache server

Initially we can start with round robin. A problem with round robin is we don't take the server load into considerations. If a server is overloaded is overloaded or slow, the LB will not stop sending new requests to that server.
To handle this, a more intelligent LB solution can be placed that periodically queries the backend server about its load and adjust the traffic based on that.
