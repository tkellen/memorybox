# memorybox ![build and test](https://github.com/tkellen/memorybox/workflows/build%20and%20test/badge.svg?branch=master) [![Coverage Status](https://coveralls.io/repos/github/tkellen/memorybox/badge.svg?branch=master)](https://coveralls.io/github/tkellen/memorybox?branch=master)
> structured digital archival. simple.

# Introduction
This project makes curating mixed digital media collections simple. In order for
the previous statement to be true, users must understand how to use the command
line on their computer. They must also understand how to read, write and
transform JSON.

As a first principal, this project expects users to be responsible for the long
term storage of their data. At the same time, it borrows ideas from distributed
storage systems like [IPFS].

The design of this software is focused on autonomous operational simplicity. No
databases. No filesystem specific features. No dependency on "the cloud". No
decentralized blockchain. Just you, a computer, a bunch of files, and enough
storage space to hold the things you've created. That's it.

## How does it work?
First, "put" some data under the management of memorybox.
```sh
➜ printf "hello world" | memorybox put -
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-04T21:55:12Z"}}

➜ memorybox put https://scaleout.team/logo.svg
{"memorybox":{"file":"b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256","source":"https://scaleout.team/logo.svg","size":6102,"importedAt":"2020-05-04T21:55:24Z"}}

➜ memorybox put *.go
{"memorybox":{"file":"e736ce2e845d3c795ccc4f924ca41c3912da9b8d63d546a33ce3b6a9ce8e0b32-sha256","source":"main.go","size":366,"importedAt":"2020-05-04T21:55:36Z"}}
{"memorybox":{"file":"58e0ceee8ed413ffe69287acf52cfed46442f1ed26622097fa1adacaf1aad06e-sha256","source":"cli.go","size":5090,"importedAt":"2020-05-04T21:55:36Z"}}
{"memorybox":{"file":"dfa7835f95a5dd5b41f5db86edef58f94b479554f06dc4241a942c42e9eae471-sha256","source":"cli_test.go","size":4352,"importedAt":"2020-05-04T21:55:36Z"}}
```

No matter where the data comes from, your imported files will end up in single
location with a flat hierarchy. By default, the destination is a folder in your
home directory called "memorybox". You can change this location, or even specify
multiple "target" stores. A new filename is computed for each file imported. No
matter how many files you import, the computed names will never conflict.
```sh
➜ find ~/memorybox -type f | sort | grep -v meta
/home/tkellen/memorybox/58e0ceee8ed413ffe69287acf52cfed46442f1ed26622097fa1adacaf1aad06e-sha256
/home/tkellen/memorybox/b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
/home/tkellen/memorybox/dfa7835f95a5dd5b41f5db86edef58f94b479554f06dc4241a942c42e9eae471-sha256
/home/tkellen/memorybox/e736ce2e845d3c795ccc4f924ca41c3912da9b8d63d546a33ce3b6a9ce8e0b32-sha256
```
> Note: the computed filenames are deterministic. The same data will always
produce the same name.

Want to import files and annotate them with data at the same time? No problem.
By default using the import function will always keep ten transfers running at
the same time. You can tweak this to be as many or as few as your computer and
network can support. This process works with ten files or ten million. You can
start and stop as often as you like and the import functionality will always
pick up where you left off.
```
➜ cat <<EOF | memorybox import -
https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg	{"flickr":{"id":"5152985571"},"name":"Nun Near Bayon in Angkor Thom"}
https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg	{"flickr":{"id":"4997667491"},"name":"Tyler Shoveling Away Sand"}
https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg	{"flickr":{"id":"4997795158"},"name":"Mongolian & Horse"}
https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg	{"name":"Vietnamese Pot-bellied Piggies","flickr":{"id":"5345981532"}}
https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg	{"flickr":{"id":"4439797619"},"name":"Tara"}
https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg	{"flickr":{"id":"4664252420"},"name":"Our Bikes in a German Forest"}
EOF
queued: 6, duplicates removed: 0, existing removed: 0
{"data":{"flickr":{"id":"4997795158"},"name":"Mongolian \u0026 Horse"},"memorybox":{"file":"635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256","source":"https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg","size":759776,"importedAt":"2020-05-07T04:37:12Z"}}
{"data":{"flickr":{"id":"4439797619"},"name":"Tara"},"memorybox":{"file":"9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256","source":"https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg","size":1967485,"importedAt":"2020-05-07T04:37:13Z"}}
{"data":{"flickr":{"id":"4997667491"},"name":"Tyler Shoveling Away Sand"},"memorybox":{"file":"661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256","source":"https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg","size":2548669,"importedAt":"2020-05-07T04:37:13Z"}}
{"data":{"name":"Vietnamese Pot-bellied Piggies","flickr":{"id":"5345981532"}},"memorybox":{"file":"2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256","source":"https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg","size":3720775,"importedAt":"2020-05-07T04:37:13Z"}}
{"data":{"flickr":{"id":"4664252420"},"name":"Our Bikes in a German Forest"},"memorybox":{"file":"43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256","source":"https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg","size":5382113,"importedAt":"2020-05-07T04:37:13Z"}}
{"data":{"flickr":{"id":"5152985571"},"name":"Nun Near Bayon in Angkor Thom"},"memorybox":{"file":"160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256","source":"https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg","size":4960170,"importedAt":"2020-05-07T04:37:14Z"}}
```

So, how do you find your files? This tool assumes the quantity of data being
dealt with is large enough that the only meaningful way to curate it is
programatically. In order to support this, a json "meta file" is created sibling
to every imported "data file".
```sh
➜ find ~/memorybox -type f | sort
/home/tkellen/memorybox/160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256
/home/tkellen/memorybox/16d8a7f2a4df6d55fab99c4db18065b28b181cf2cf7c9bc9e3293eb4b7214b3f-sha256
/home/tkellen/memorybox/2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256
/home/tkellen/memorybox/43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256
/home/tkellen/memorybox/635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256
/home/tkellen/memorybox/661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256
/home/tkellen/memorybox/737bc0bc21c2a00e6461d25171ce4926bccabaa9bf9c6979ee13450528745796-sha256
/home/tkellen/memorybox/9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256
/home/tkellen/memorybox/b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/b3f80e041b6bd8c266f6ad4981f81d476e02fc963b54cc8df2e28965b81a2b39-sha256
/home/tkellen/memorybox/b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
/home/tkellen/memorybox/memorybox-meta-160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256
/home/tkellen/memorybox/memorybox-meta-16d8a7f2a4df6d55fab99c4db18065b28b181cf2cf7c9bc9e3293eb4b7214b3f-sha256
/home/tkellen/memorybox/memorybox-meta-2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256
/home/tkellen/memorybox/memorybox-meta-43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256
/home/tkellen/memorybox/memorybox-meta-635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256
/home/tkellen/memorybox/memorybox-meta-661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256
/home/tkellen/memorybox/memorybox-meta-737bc0bc21c2a00e6461d25171ce4926bccabaa9bf9c6979ee13450528745796-sha256
/home/tkellen/memorybox/memorybox-meta-9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256
/home/tkellen/memorybox/memorybox-meta-b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256
/home/tkellen/memorybox/memorybox-meta-b3f80e041b6bd8c266f6ad4981f81d476e02fc963b54cc8df2e28965b81a2b39-sha256
/home/tkellen/memorybox/memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256
```

Initially, meta files hold minimal data about your imported files. Where they
came from. How big they are. That sort of thing. Users are expected to annotate
their files with data that makes them consumable by other systems (e.g. static
site generators).
```sh
➜ memorybox meta b94d27 | jq
{
  "memorybox": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "source": "-",
    "size": 11,
    "importedAt": "2020-05-05T14:30:07Z"
  }
}
➜ memorybox meta set b94d27 demo value | jq
{
  "memorybox": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "source": "-",
    "size": 11,
    "importedAt": "2020-05-05T14:30:07Z"
  }
  "data": {
    "demo": "value"
  }
}
➜ memorybox meta delete b94d27 demo | jq
{
  "memorybox": {
    "file": "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256",
    "source": "-",
    "size": 11,
    "importedAt": "2020-05-05T14:30:07Z"
  }
  "data": {}
}
```

All meta files can be viewed by generating an index.
```sh
➜ memorybox index
{"memorybox":{"file":"b217de9d6cd699575ea4981761d21c9d107424d11e058cac784ad90d63e5cbe7-sha256","source":"https://scaleout.team/logo.svg","size":6102,"importedAt":"2020-05-07T04:41:55Z"}}
{"memorybox":{"file":"b3f80e041b6bd8c266f6ad4981f81d476e02fc963b54cc8df2e28965b81a2b39-sha256","source":"cli_test.go","size":5194,"importedAt":"2020-05-07T04:41:57Z"}}
{"data":{"name":"Vietnamese Pot-bellied Piggies","flickr":{"id":"5345981532"}},"memorybox":{"file":"2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256","source":"https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg","size":3720775,"importedAt":"2020-05-07T04:42:10Z"}}
{"memorybox":{"file":"737bc0bc21c2a00e6461d25171ce4926bccabaa9bf9c6979ee13450528745796-sha256","source":"main.go","size":319,"importedAt":"2020-05-07T04:41:57Z"}}
{"data":{"flickr":{"id":"4664252420"},"name":"Our Bikes in a German Forest"},"memorybox":{"file":"43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256","source":"https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg","size":5382113,"importedAt":"2020-05-07T04:42:10Z"}}
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-07T04:41:52Z"},"data":{}}
{"data":{"flickr":{"id":"5152985571"},"name":"Nun Near Bayon in Angkor Thom"},"memorybox":{"file":"160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256","source":"https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg","size":4960170,"importedAt":"2020-05-07T04:42:11Z"}}
{"data":{"flickr":{"id":"4997795158"},"name":"Mongolian \u0026 Horse"},"memorybox":{"file":"635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256","source":"https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg","size":759776,"importedAt":"2020-05-07T04:42:09Z"}}
{"data":{"flickr":{"id":"4997667491"},"name":"Tyler Shoveling Away Sand"},"memorybox":{"file":"661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256","source":"https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg","size":2548669,"importedAt":"2020-05-07T04:42:10Z"}}
{"memorybox":{"file":"16d8a7f2a4df6d55fab99c4db18065b28b181cf2cf7c9bc9e3293eb4b7214b3f-sha256","source":"cli.go","size":7062,"importedAt":"2020-05-07T04:41:57Z"}}
{"data":{"flickr":{"id":"4439797619"},"name":"Tara"},"memorybox":{"file":"9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256","source":"https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg","size":1967485,"importedAt":"2020-05-07T04:42:10Z"}}
```

Any tool that supports JSON can filter the index.
```sh
➜ memorybox index | jq -c 'select(.memorybox.source | endswith("jpg"))'
{"data":{"flickr":{"id":"5152985571"},"name":"Nun Near Bayon in Angkor Thom"},"memorybox":{"file":"160b7f0b12cdee794db30427ecceb8429e5d8fb2c2aff7f12ccacdf1fadc357b-sha256","source":"https://live.staticflickr.com/4018/5152985571_1f6631bca8_o.jpg","size":4960170,"importedAt":"2020-05-07T04:42:11Z"}}
{"data":{"name":"Vietnamese Pot-bellied Piggies","flickr":{"id":"5345981532"}},"memorybox":{"file":"2bcd21b5919ef74a1a8b9d5167b8488f5f8707abbaaa81fc20b17174ddb1363e-sha256","source":"https://live.staticflickr.com/5044/5345981532_0ec5bbff9f_o.jpg","size":3720775,"importedAt":"2020-05-07T04:42:10Z"}}
{"data":{"flickr":{"id":"4439797619"},"name":"Tara"},"memorybox":{"file":"9ef1e9090b4a34d427c24a2a75a50148a8dc1bcef683c674704ac7cf1d771585-sha256","source":"https://live.staticflickr.com/4016/4439797619_bccc764fe8_o.jpg","size":1967485,"importedAt":"2020-05-07T04:42:10Z"}}
{"data":{"flickr":{"id":"4997667491"},"name":"Tyler Shoveling Away Sand"},"memorybox":{"file":"661a7dcf47c087403ca5981b58f48b7713cdf1dc49fe2036cb62fc1902e8ba9a-sha256","source":"https://live.staticflickr.com/4132/4997667491_794d24a3d5_o.jpg","size":2548669,"importedAt":"2020-05-07T04:42:10Z"}}
{"data":{"flickr":{"id":"4664252420"},"name":"Our Bikes in a German Forest"},"memorybox":{"file":"43781812980cce2da36c42a002ca09a37de0c49865a339631f11a211fba059b9-sha256","source":"https://live.staticflickr.com/4034/4664252420_6172670bf6_o.jpg","size":5382113,"importedAt":"2020-05-07T04:42:10Z"}}
{"data":{"flickr":{"id":"4997795158"},"name":"Mongolian & Horse"},"memorybox":{"file":"635bac5142e7de86a2943fcbec9e57f022d82b6e298de394fde49a65b8a33eec-sha256","source":"https://live.staticflickr.com/4128/4997795158_733eb79733_o.jpg","size":759776,"importedAt":"2020-05-07T04:42:09Z"}}
```

Generating the index detects missing meta files.
```sh
➜ printf "hello world" | memorybox put -
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-06T21:48:10Z"}}
➜ rm ~/memorybox/memorybox-meta-b94d27*
➜ memorybox index
store corrupted: datafile b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing metafile memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256%
```

Generating the index detects missing data files.
```sh
➜ printf "hello world" | memorybox put -
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-06T21:48:10Z"}}
➜ rm ~/memorybox/b94d27*
➜ memorybox index
store corrupted: metafile memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 missing datafile b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256%
```

Generating the index detects corrupted meta files.
```sh
➜ printf "hello world" | memorybox put -
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-06T21:48:10Z"}}
➜ printf "junk" > ~/memorybox/memorybox-meta-b94d27*
➜ memorybox index
store corrupted: memorybox.file key in memorybox-meta-b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 conflicts with filename%
```

Generating the index can detect corrupted data files (if you ask it to).
```sh
➜ printf "hello world" | memorybox put -
{"memorybox":{"file":"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256","source":"-","size":11,"importedAt":"2020-05-06T21:48:10Z"}}
➜ printf "junk" > ~/memorybox/b94d27*
➜ memorybox index rehash
store corrupted: b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9-sha256 should be named ef875a1705a5fdac206be996f4dc1f726ea6b68861eb741c37def7277f179e37-sha256, possible data corruption
```
> Note: Depending on the size and location of the store (local vs remote) using
the "rehash" function can take some time as it requires processing every single
bit of every single file in the store.

You can also modify the index and re-import it, updating every meta file with a
single command. It is also possible to modify meta files using any external
process you desire.
```sh
NOT YET IMPLEMENTED.
```

There is no visual mechanism for viewing what you have stored. It is up to you
to build something to showcase it. I use this tool to support authoring a media
heavy website that is distributed via USB thumb drives.

## Benefits
Data can be categorized and queried using any tool that interacts with JSON.

The only system requirement for this software is free storage space.

Checking the integrity of your data is trivial.

Accidentally importing the same thing more than once is automatically detected
and prevented.

Making backup copies is very easy. Just copy the folder to another device.

## Drawbacks
Changing even a single bit in any file and trying to import it again will cause
the entire file to be duplicated. There are many highly technical approaches to
eliminating this constraint. All of them violate the primary design goal of
simplicity for this project. For maximum usability, files you manage with
memorybox should be considered "done", ideally forever. Think of it like a real,
physical memorybox.

## Try it
Clone this repo and run the following:

```sh
go build && ./memorybox
```

### Prior Art (in order of my becoming aware of them)
* [Scuttlebutt](https://scuttlebutt.nz/)
* [IPFS]
* [Dat](https://dat.foundation/)
* [Filecoin](https://filecoin.io/)
* [Perkeep](https://perkeep.org/)
* [casync](https://github.com/systemd/casync)
* [Storj](https://storj.io/)
* [SAFE Network](https://safenetwork.tech/)

[jsonl]: http://jsonlines.org/
[IPFS]: https://ipfs.io/