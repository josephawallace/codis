# 🔐 Codis

Secure cryptocurrency custody solution using MPC.

## Setup

Getting a development environment ready for Codis is quick and easy. First install [Go](https://go.dev/dl/). Then, clone
the repository down and run `go mod tidy` in the project root to install the needed dependencies. Now you're ready to
Go ;)

## Keygen

Currently, only (local) distributed key generation is implemented in Codis. The different participants of the keygen 
protocol run on separate threads via [Goroutines](https://gobyexample.com/goroutines) and communicate their messages 
using [Go Channels](https://gobyexample.com/channels). They follow the protocol described in 
[[GG18]](https://eprint.iacr.org/2019/114.pdf).
