# memorybox
> easy to understand, content-addressed, digital archival

# Introduction
On the shoulders of a lot of really cool projects I am teaching myself how to
make a content addressable storage engine. At the time of this writing, I am
aiming to keep the implementation to less than 500 lines of code (not including
comments or tests).

It will be possible to use local disk or any storage service that provides a
s3-compatible API to ensure the bits that go in stand the test of time.

I will probably implement some kind of adapter for all of the projects listed
below. 

## Non-Goals
* Being a generalized point-in-time backup solution for in-progress creations.
* Making it easy to manage absolutely every digital artifact one has produced.

## Goals
Imagine a memory box you might find or make in real life. It likely contains
carefully selected objects of personal sentimental value. These are the types of
things this project is designed to make easy to store and retrieve.
 
## Try it
Clone this repo and run the following:
```sh
go run ./... local save $(find -type f)
```
...this will copy every file in the repo (including files under `.git`) to
`~/memorybox`, naming each file the sha256 sum of its contents.

## Prior Art
* [Perkeep](https://perkeep.org/)
* [IPFS](https://ipfs.io/)
* [Scuttlebutt](https://scuttlebutt.nz/)
* [Dat](https://dat.foundation/)