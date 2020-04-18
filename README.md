# memorybox ![build and test](https://github.com/tkellen/memorybox/workflows/build%20and%20test/badge.svg?branch=master)
> easy to understand digital archival tool

# Introduction
On the shoulders of a lot of really cool projects I am teaching myself how to
make a content addressable storage engine. At the time of this writing, I am
aiming to keep the command line interface to less than 1000 readable lines of
code with 100% test coverage (usage docs / comments / tests not included in
count).

## Goals
Imagine a memory box you might find or make in real life. It likely contains
carefully selected objects of personal sentimental value. These are the types of
digital artifacts this project is designed to make easy to catalog. Separate
projects will be created to showcase the contents within.

## Non-Goals
* Being a generalized point-in-time backup solution for in-progress creations.
* Making it easy to manage absolutely every digital artifact one has produced.
* Providing a UI to interact with files that have been stored.

## Try it
Clone this repo and run the following:
```sh
go build && ./memorybox
```

## Prior Art
* [Perkeep](https://perkeep.org/)
* [IPFS](https://ipfs.io/)
* [Scuttlebutt](https://scuttlebutt.nz/)
* [Dat](https://dat.foundation/)

## Final Thoughts
At some level, this project is essentially an over-engineered version of this:
```
aws s3 cp file s3://bucket/sha256-$((sha256sum < file) | cut -d' ' -f1)
```
...or, the '57 Chevy version of Perkeep.