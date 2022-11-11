# 🔐 Codis

## What?

Secure cryptocurrency custody using MPC.

## Why?

**Security**. Secure multi-party computation (also known as secure computation, multi-party computation (MPC) or privacy-preserving
computation) is a subfield of cryptography with the goal of creating methods for parties to jointly compute a function
over their inputs while keeping those inputs private. Unlike traditional cryptographic tasks, where cryptography
assures security and integrity of communication or storage and the adversary is outside the system of participants (an
eavesdropper on the sender and receiver), the cryptography in this model protects participants' privacy from each other.<sup>1</sup>
In the context of crypto custody, MPC is usually used to create shards of a private key that are each held by
designated parties. These parties can use these shards as input to a signature function (i.e. ECDSA).
On computing a signature, the inputted shards are not revealed to any parties and the underlying secret is never created
in whole. This means that the private key is _never_ known by any individual or machine. Therefore, the security of a
key is equivalent to the probability that a designated quorum of participants stays faithful and does not collude.
Contrast this with cold-custody,<sup>2</sup> where the security of the key is based on its physical isolation (typically
by using HSMs that are disconnected from the internet).

**Convenience**. Immediately, there are huge development/deployment conveniences gained by using MPC over cold-custody. Where HSMs are
cumbersome to work with due to their proprietary and isolated nature, MPC solutions can follow development/deployment
cycles that are much more similar to that of "regular" software. Developers are not expected to learn arcane libraries
and administrators are not expected to travel to remote bunkers for maintenance or disaster recovery.
Due to it being less clunky and (arguably) safer than cold storage, institutions have pioneered the deployment of MPC.
However, it is our opinion that MPC will have an even greater impact on the lay blockchain consumer and how they
interact with their assets. As such, the development decisions made while designing Codis were meant to satisfy the
broad use cases&mdash;perhaps favoring convenience over low-level control/exposure, but never sacrificing security.

## How?

### Environment

Getting a development environment ready for Codis is quick and easy. First install [Go](https://go.dev/dl/). Then, clone
this repository down and run `go mod tidy` in the project root to install the needed dependencies. Now you're ready to
Go ;)

### Peer Configuration

The _configs/config.yaml_ file contains all the configuration details for the bootstrap, service, and client nodes that
Codis needs to run. Each key within the `peers` list is treated as the peer's application ID. It is this name that is
specified on the command line when you want to run a peer.

Aside from the applications ID for each node, every _libp2p_ peer has a private key that is used to prove its identity 
on the network. Codis will map the application IDs to each node's corresponding private key file (which can be found in
the `keys` directory). If the private key file for that ID does not exist, it will be created.

The IP/port a node should listen on, any host it should connect to, the bootstrap nodes to connect to, or the PSK for the 
network it wants to join, are all other peer details that may be useful to specify in the config.

## Keygen

Currently, only (local) distributed key generation is implemented in Codis. The different participants of the keygen
protocol run on separate threads via [Goroutines](https://gobyexample.com/goroutines) and communicate their messages
using [Go Channels](https://gobyexample.com/channels). They follow the protocol described in
[[GG18]](https://eprint.iacr.org/2019/114.pdf).
