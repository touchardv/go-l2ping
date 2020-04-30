# go-l2ping

A quick and dirty PoC conversion to the Go language of the `l2ping` Bluetooth utility command (part of the Linux BlueZ stack).

## Building

Requirements:

* The `make` command (e.g. [GNU make](https://www.gnu.org/software/make/manual/make.html)).
* The [Golang toolchain](https://golang.org/doc/install) (version 1.13 or later).

In a shell, execute: `make` (or `make build`).

The build artifacts can be cleaned by using: `make clean`.

## Running

`go-l2ping AA:BB:CC:DD:EE`
