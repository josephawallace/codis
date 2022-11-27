FROM golang:1.19-bullseye

WORKDIR /root

RUN git config --global url."git@github.com".insteadOf "https://github.com"

RUN go env -w GOPRIVATE=github.com/milquellc/codis
