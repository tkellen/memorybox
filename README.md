# memorybox ![build and test](https://github.com/tkellen/memorybox/workflows/build%20and%20test/badge.svg?branch=master)
> easy to understand digital archival tool

# Introduction
On the shoulders of a lot of really cool projects I am teaching myself how to
make a content addressable storage engine. At the time of this writing, I am
aiming to keep the implementation to less than 500 readable lines of code (not
including comments or tests).

It will be possible to use local disk or any storage service that provides a
s3-compatible API to ensure the bits that go in stand the test of time.

I will probably implement some kind of adapter for all of the projects listed
in the prior art section.

## Non-Goals
* Being a generalized point-in-time backup solution for in-progress creations.
* Making it easy to manage absolutely every digital artifact one has produced.
* Providing a UI to interact with files that have been stored.

## Goals
Imagine a memory box you might find or make in real life. It likely contains
carefully selected objects of personal sentimental value. These are the types of
digital artifacts this project is designed to make easy to catalog. Separate
projects will be created to showcase the contents within.
 
## Try it
Clone this repo and run the following:
```sh
go build
echo "wat" | ./memorybox -d put local -
./memorybox -d get local sha256-19a
./memorybox -d put local https://scaleout.team/logo.svg
./memorybox -d put local $(find -type f)
```

## Prior Art
* [Perkeep](https://perkeep.org/)
* [IPFS](https://ipfs.io/)
* [Scuttlebutt](https://scuttlebutt.nz/)
* [Dat](https://dat.foundation/)