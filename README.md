## Running the Certificate Authority
To run the certificate authority, navigate to `Partage-CA` and run the file `run.sh`.
## Running the Client
To run the client, please navigate to `Partage-Client/peer/main` and run
```bash
$ go run main.go -port=PORT -id=ID -i=INTRODUCER_ADDRESS
```
Here, PORT denotes the port of the frontend, ID denotes the Paxos ID of the client, and INTRODUCER_ADDRESS denotes another node already in the system.
The first node in the system will have its INTRODUCER_ADDRESS as an empty string, and an ID of 1. The other nodes can use that node's Partage IP address 
(displayed at the command line after running the client) as INTRODUCER_ADDRESS and with increasing IDs.
## Example
Here is an example for running the system with three nodes. Make sure that the certificate authority is already running in the background.
We also assume that the IP address of the node 1 is `127.0.0.1:5738`. In your case, you should use the IP address displayed on the console after
starting node 1.
#### Node 1
```bash
$ go run main.go -port=8000 -id=1 -i=
```
#### Node 2
```bash
$ go run main.go -port=8001 -id=2 -i=127.0.0.1:5738
```
#### Node 3
```bash
$ go run main.go -port=8002 -id=3 -i=127.0.0.1:5738
```
Then, for example, to navigate to the corresponding frontend of node 1, open `127.0.0.1:8000` in your browser.
