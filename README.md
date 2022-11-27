# 🔐 Codis

Secure cryptocurrency custody using MPC.

## Getting Started

Obviously, we need multiple parties to demonstrate or use MPC. In order to simulate a network of parties on our local
machines, we use Docker Compose. You can find more information on how to install Docker [here](https://docs.docker.com/get-docker/).

Once Docker is installed, you can spin up the first node on your local Codis network by running `docker compose up bootstrap`. 
Record the address the bootstrap node is listening at, as we will use it later. Now, close the bootstrap and run it again
in daemon mode by adding a `-d` flag to the end of the original command.

Start a cluster of peers by running `CODIS_BOOTSTRAPS=<bootstrap_address> docker compose up peer`. Again, take note of 
the peer addresses (at least one) as they will be used later.

> The number of peers that will start is defined by the number of deploy replicas specified in the *docker-compose.yaml* file.

Finally, in another terminal, create a client by running `CODIS_BOOTSTRAPS=<bootstrap_address> CODIS_HOST=<peer_address> docker compose up client`.
You should see that the client has connected to the host and a terminal prompt should appear.

🚨 *The client terminal interface is currently not working through Docker. You can still send requests to the network by
setting the necessary environment variables on the local machine and running the client outside of Docker (refer to 
[CODIS-T-23]()).* 🚨

### Node Roles

At this point, you might be wondering what the *bootstrap* service is. Why did we run a bootstrap service first instead of
a *peer* service or a *client* service? To understand this, it is first important to know that Codis is a peer-to-peer 
application (using [libp2p](https://github.com/libp2p/specs)) and that the peers can have roles that they serve within the network.

#### Bootstrap Service

The bootstrap service starts a bootstrap node that is used to keep and share a running list of any peers that connect to it. 
In this way, joining peers can quickly learn about the network if they connect to a bootstrap.

#### Peer Service

The peer service starts a set of peers that can execute the Codis protocols. These peers communicate with each other to
generate the keys or signatures that are requested.

#### Client Service

The client service starts a client node that can send RPC calls to its specified host peer. The client node will form and
send a request for generating keys or signatures.

### Configuration

Codis uses [Viper]() to source configurations from flags, files, and environment variables. Because of this, it can 
become a bit confusing on how a node is being configured&mdash;especially when running through Docker. The most important
thing to remember is the precedence order for configuration: (1) flags, (2) environment variables, then (3) config file.

For development outside of Docker, it's easiest to not set environment variables and instead edit the *config.yaml* file. 
For development with Docker, it is necessary to configure services using environment variables&mdash;which you can edit
for each service within the *docker-compose.yaml* file.
